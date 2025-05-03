package blockchain

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	// "go-api-template/internal/services" // We could have a service to store data from the event
)

// AnswerUpdatedEvent holds the unpacked data for the AnswerUpdated event.
type AnswerUpdatedEvent struct {
	Current   *big.Int   // Indexed - from Topics[1]
	RoundId   *big.Int   // Indexed - from Topics[2]
	UpdatedAt *big.Int   // Not Indexed - from Data
	Raw       types.Log // Optionally keep raw log
}

type EventListener struct {
	client         *ethclient.Client
	contractAddr   common.Address
	contractAddrAgg common.Address
	contractABI    abi.ABI
	// service *services.BlockchainService // Example service to handle data
	stopChan       chan struct{}
	wg             sync.WaitGroup
	logger         *log.Logger
	rpcURL         string // Store for potential reconnection
	filterQuery    ethereum.FilterQuery
	eventSignature common.Hash // Store the signature hash for the event we care about
}

// NewEventListener creates and initializes the listener
func NewEventListener(rpcURL, contractAddrHex, abiPath string /*, other services */) (*EventListener, error) {
	logger := log.New(os.Stdout, "[Blockchain] ", log.LstdFlags|log.Lshortfile) // Added Lshortfile for debugging

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client at %s: %w", rpcURL, err)
	}
	logger.Printf("Connected to Ethereum node: %s", rpcURL)

	contractAddr := common.HexToAddress(contractAddrHex)

	// Resolve absolute path for ABI if it's relative
	absAbiPath := abiPath
	if !filepath.IsAbs(absAbiPath) {
		// Assuming relative path is from project root
		wd, err := os.Getwd()
		if err != nil {
			client.Close() // Close client if we fail early
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		absAbiPath = filepath.Join(wd, absAbiPath)
	}

	logger.Printf("Attempting to read ABI file: %s", absAbiPath)
	abiBytes, err := os.ReadFile(absAbiPath)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to read ABI file '%s': %w", absAbiPath, err)
	}

	contractABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to parse contract ABI from '%s': %w", absAbiPath, err)
	}

	// Get the specific event signature we want to listen for
	eventName := "AnswerUpdated" //Example event with price feed data just for demo
	eventABI, ok := contractABI.Events[eventName]
	if !ok {
		client.Close()
		return nil, fmt.Errorf("event '%s' not found in ABI file '%s'", eventName, absAbiPath)
	}
	eventSignature := eventABI.ID
	logger.Printf("Targeting event '%s' with signature: %s", eventName, eventSignature.Hex())

	methodName := "aggregator"
	// Pack the method call (no arguments for 'aggregator')
	callData, err := contractABI.Pack(methodName)
	if err != nil {
		log.Fatalf("Failed to pack data for %s: %v", methodName, err)
	}

	// Make the call
	log.Printf("Calling method '%s' on contract %s...", methodName, contractAddrHex)
	resultBytes, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: callData,
	}, nil) // nil for latest block
	if err != nil {
		log.Fatalf("Failed to call contract method %s: %v", methodName, err)
	}

	// Unpack the result - the 'aggregator' method returns a single address
	var aggregatorAddress common.Address
	err = contractABI.UnpackIntoInterface(&aggregatorAddress, methodName, resultBytes)
	if err != nil {
		results, errAlt := contractABI.Unpack(methodName, resultBytes)
		if errAlt != nil || len(results) == 0 {
			log.Fatalf("Failed to unpack %s result (tried both ways): %v / %v", methodName, err, errAlt)
		}
		var ok bool
		aggregatorAddress, ok = results[0].(common.Address)
		if !ok {
			log.Fatalf("Failed type assertion for %s result: expected common.Address, got %T", methodName, results[0])
		}
	}

	// Prepare the filter query, filtering only for the specific event
	query := ethereum.FilterQuery{
		Addresses: []common.Address{aggregatorAddress, contractAddr},
		Topics: [][]common.Hash{
			{eventSignature}, // Only listen for logs where Topics[0] matches our event signature
		},
	}

	return &EventListener{
		client:         client,
		contractAddr:   contractAddr,
		contractAddrAgg: aggregatorAddress,
		contractABI:    contractABI,
		eventSignature: eventSignature,
		// service: service,
		stopChan:     make(chan struct{}),
		logger:       logger,
		rpcURL:       rpcURL, // Store for potential reconnection
		filterQuery:  query,
	}, nil
}

// Start begins listening for events in a separate goroutine
func (l *EventListener) Start(ctx context.Context) {
	l.wg.Add(1)
	go l.listenLoop(ctx)
	l.logger.Printf("Started event listener for contract: %s, Event: AnswerUpdated", l.contractAddrAgg.Hex())
}

// Stop signals the listener to shut down and waits for it to complete
func (l *EventListener) Stop() {
	l.logger.Printf("Stopping event listener for contract: %s", l.contractAddrAgg.Hex())
	close(l.stopChan) // Signal the loop to stop
	l.wg.Wait()       // Wait for the loop goroutine to finish
	l.client.Close()  // Close the connection
	l.logger.Printf("Event listener stopped.")
}

// listenLoop is the main event subscription loop with reconnection logic
func (l *EventListener) listenLoop(ctx context.Context) {
	defer l.wg.Done() // Signal that this goroutine has finished when it exits

	var sub ethereum.Subscription
	var logs chan types.Log

	connectAndSubscribe := func(loopCtx context.Context) error {
		var err error
		// Attempt reconnection if client is nil or connection is lost
		if l.client == nil {
			l.logger.Println("Attempting to reconnect client...")
			l.client, err = ethclient.DialContext(loopCtx, l.rpcURL)
			if err != nil {
				return fmt.Errorf("reconnection failed: %w", err)
			}
			l.logger.Println("Reconnected to Ethereum node.")
		}

		logs = make(chan types.Log, 10) // Buffered channel
		l.logger.Println("Attempting to subscribe to logs...")
		l.logger.Printf("Listening for events on contracts: %v\n", l.filterQuery.Addresses)
		sub, err = l.client.SubscribeFilterLogs(loopCtx, l.filterQuery, logs)
		if err != nil {
			l.client.Close() // Close potentially bad connection
			l.client = nil   // Mark client as nil for next attempt
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		l.logger.Println("Subscription active. Waiting for events...")
		return nil
	}

	// Initial connection attempt
	if err := connectAndSubscribe(ctx); err != nil {
		l.logger.Printf("ERROR: Initial connection/subscription failed: %v. Listener will not run.", err)
		return // Exit if initial connection fails
	}

	reconnectDelay := 5 * time.Second // Initial delay

	for {
		select {
		case <-l.stopChan:
			l.logger.Println("Received stop signal, shutting down listener loop.")
			if sub != nil {
				sub.Unsubscribe()
			}
			return
		case <-ctx.Done():
			l.logger.Println("Context cancelled, shutting down listener loop.")
			if sub != nil {
				sub.Unsubscribe()
			}
			return
		case err := <-sub.Err():
			l.logger.Printf("ERROR: Subscription error: %v. Attempting to reconnect...", err)
			sub.Unsubscribe() // Unsubscribe from the broken subscription
			if l.client != nil {
				l.client.Close() // Close the client connection
			}
			l.client = nil    // Ensure Dial is called on next attempt
			sub = nil         // Mark subscription as inactive

			select {
			case <-time.After(reconnectDelay):
				if err := connectAndSubscribe(ctx); err != nil {
					l.logger.Printf("ERROR: Re-subscription failed: %v. Retrying after %v.", err, reconnectDelay)
				} else {
					reconnectDelay = 5 * time.Second // Reset delay on successful reconnect
				}
			case <-l.stopChan:
				l.logger.Println("Stop signal received during reconnect delay.")
				return
			case <-ctx.Done():
				l.logger.Println("Context cancelled during reconnect delay.")
				return
			}
		case vLog := <-logs:
			if len(vLog.Topics) > 0 && vLog.Topics[0] == l.eventSignature {
				l.logger.Printf("Received log: Block %d, Tx %s", vLog.BlockNumber, vLog.TxHash.Hex())
				l.handleAnswerUpdated(vLog)
			} else {
				l.logger.Printf("WARN: Received unexpected log signature: %s (Expected: %s)", vLog.Topics[0].Hex(), l.eventSignature.Hex())
			}
		}
	}
}

// handleAnswerUpdated specifically parses the AnswerUpdated event
func (l *EventListener) handleAnswerUpdated(vLog types.Log) {
	eventName := "AnswerUpdated"
	eventABI, ok := l.contractABI.Events[eventName]
	if !ok {
		// This should ideally not happen as we check in NewEventListener, but good practice
		l.logger.Printf("CRITICAL: ABI definition for event '%s' not found during handling.", eventName)
		return
	}

	var eventData AnswerUpdatedEvent
	eventData.Raw = vLog // Store raw log

	// --- Unpack Indexed Fields from Topics ---
	if len(vLog.Topics) < 3 { // This demo event should have at least 3 topics
		l.logger.Printf("ERROR: Expected at least 3 topics for %s, got %d. Log: %+v", eventName, len(vLog.Topics), vLog)
		return
	}
	eventData.Current = new(big.Int).SetBytes(vLog.Topics[1].Bytes())
	eventData.RoundId = new(big.Int).SetBytes(vLog.Topics[2].Bytes())

	// --- Unpack Non-Indexed Fields from Data ---
	nonIndexedArgs := eventABI.Inputs.NonIndexed()
	if len(nonIndexedArgs) == 0 && len(vLog.Data) > 0 {
		l.logger.Printf("WARN: Event %s has non-empty data (%x) but no non-indexed arguments in ABI", eventName, vLog.Data)
		// If only indexed fields are needed, we might continue here.
		// For AnswerUpdated, updatedAt is crucial, so we likely should return or handle differently.
	} else if len(nonIndexedArgs) > 0 {
		// Unpack the non-indexed fields from vLog.Data
		unpackedData, err := nonIndexedArgs.Unpack(vLog.Data)
		if err != nil {
			l.logger.Printf("ERROR: Failed to unpack non-indexed data for %s: %v. Data: %x", eventName, err, vLog.Data)
			return
		}

		// Assign the unpacked values. On the demo event there should only be updatedAt
		if len(unpackedData) > 0 {
			var ok bool
			eventData.UpdatedAt, ok = unpackedData[0].(*big.Int) // Type assertion
			if !ok {
				l.logger.Printf("ERROR: Type assertion failed for non-indexed argument 'updatedAt' (expected *big.Int). Value: %+v", unpackedData[0])
				return // Stop processing if the type is wrong
			}
		} else {
			l.logger.Printf("WARN: Unpack returned empty slice for non-indexed args of %s, though ABI defines them.", eventName)
		}
	} else if len(vLog.Data) > 0 {
		// ABI has no non-indexed args, but data is present. Log it.
		l.logger.Printf("INFO: Event %s has data (%x) but no non-indexed arguments defined in ABI.", eventName, vLog.Data)
	}

	// Check if UpdatedAt was successfully unpacked if it's required
	if eventData.UpdatedAt == nil {
		l.logger.Printf("ERROR: Failed to obtain 'updatedAt' value for event %s. Log: %+v", eventName, vLog)
	}


	l.logger.Printf("Successfully unpacked %s: Current=%s, RoundId=%s, UpdatedAt=%s (Block: %d)",
		eventName,
		eventData.Current.String(),
		eventData.RoundId.String(),
		eventData.UpdatedAt.String(), // Use the unpacked value
		vLog.BlockNumber,
	)

	// If implemented, a service to handle the data, we could call it here
	fmt.Printf("==> ACTION: Handle %s Event - Price: %s, Time: %s\n",
		eventName, eventData.Current.String(), eventData.UpdatedAt.String())


	// If we have payments through a smart contract for example, we could automatically update invoice state based on on-chain events
	// Check a specific event with the topic InvoicePayment for example
	// Either the event would have an invoice ID associated or we use the wallets involved in the transaction
	// Contractor or Employer would have their own wallets and we could check the event for those
}