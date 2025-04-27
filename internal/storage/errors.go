package storage

import "errors"

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("resource conflict (e.g., duplicate key)")