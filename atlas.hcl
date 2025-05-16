env "local" {
  src = "file://schema.sql"
  url = "postgres://postgres:postgres@localhost:5433/api_db?sslmode=disable"
  dev = "postgres://postgres:postgres@localhost:5433/api_db_atlas?sslmode=disable"
  migration {
    dir    = "file://internal/database/migrations"
    format = golang-migrate
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}