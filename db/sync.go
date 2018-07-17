package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

func applyAddTable(transaction *sql.Tx, params AddTableParams) error {

	if strings.TrimSpace(params.Name) == "" {
		return fmt.Errorf("table is required")
	}

	query := fmt.Sprintf("CREATE TABLE \"%v\" ();", params.Name)
	_, err := transaction.Exec(query)
	if err != nil {
		return fmt.Errorf("can't create table %v: %v/n", params.Name, err)
	}

	return nil
}

func applyDeleteTable(transaction *sql.Tx, params DeleteTableParams) error {

	if strings.TrimSpace(params.Name) == "" {
		return fmt.Errorf("table is required")
	}

	query := fmt.Sprintf("DROP TABLE \"%v\"", params.Name)
	_, err := transaction.Exec(query)

	if err != nil {
		return fmt.Errorf("can't delete table %v: %v/n", params.Name, err)
	}

	return nil
}

func applyAddColumn(transaction *sql.Tx, params AddColumnParams) error {

	if strings.TrimSpace(params.Table) == "" {
		return fmt.Errorf("table is required")
	}

	if strings.TrimSpace(params.Column) == "" {
		return fmt.Errorf("column is required")
	}

	columnType := params.Type
	notNullParam := ""
	if !params.IsNullable {
		notNullParam = "NOT NULL"
	}

	defaultValueParam := ""
	if params.DefaultValue != "" {
		defaultValueParam = fmt.Sprintf("DEFAULT '%v';", params.DefaultValue)
	}

	query := fmt.Sprintf(`
		ALTER TABLE "%v"
			ADD COLUMN "%v" %v %v %v
	`, params.Table, params.Column, columnType, notNullParam, defaultValueParam)

	_, err := transaction.Exec(query)
	if err != nil {
		return fmt.Errorf("can't add column '%v' to table '%v': %v/n", params.Column, params.Table, err)
	}

	return nil
}

func applyDeleteColumn(transaction *sql.Tx, params DeleteColumnParams) error {

	query := fmt.Sprintf(`
		ALTER TABLE "%v"
			DROP COLUMN "%v"
	`, params.Table, params.Column)

	_, err := transaction.Exec(query)
	if err != nil {
		return fmt.Errorf("can't delete column '%v' at table '%v': %v/n", params.Column, params.Table, err)
	}

	return nil
}

func applyAddPrimaryKey(transaction *sql.Tx, migrationId string, actionIndex int, params AddPrimaryKeyParams) error {

	snapshot, err := GetSnapshotWithAction(migrationId, actionIndex)
	if err != nil {
		return err
	}

	table := getTableFromSnapshot(snapshot, params.Table)
	if table == nil {
		return fmt.Errorf("table '%v' doesn't exist", params.Table)
	}

	column := getColumnFromTable(table, params.Column)
	if column == nil {
		return fmt.Errorf("column '%v' doesn't exist", params.Column)
	}

	if len(table.PrimaryKeys) > 1 {
		query := fmt.Sprintf(`
			ALTER TABLE "%v"
				DROP CONSTRAINT pkey
		`, params.Table)

		_, err := transaction.Exec(query)
		if err != nil {
			return err
		}
	}

	keys := ""
	for index, key := range table.PrimaryKeys {
		if index == 0 {
			keys = fmt.Sprintf(`"%v"`, key)
		} else {
			keys += fmt.Sprintf(`, "%v"`, key)
		}

	}

	query := fmt.Sprintf(`
		ALTER TABLE "%v"
			ADD CONSTRAINT pkey PRIMARY KEY (%v);
	`, params.Table, keys)

	_, err = transaction.Exec(query)
	if err != nil {
		return fmt.Errorf("can't add primary key '%v' to table '%v': %v/n", params.Column, params.Table, err)
	}

	return nil
}

func applyDeletePrimaryKey(transaction *sql.Tx, migrationId string, actionIndex int, params DeletePrimaryKeyParams) error {

	snapshot, err := GetSnapshotWithAction(migrationId, actionIndex)
	if err != nil {
		return err
	}

	table := getTableFromSnapshot(snapshot, params.Table)
	if table == nil {
		return fmt.Errorf("table '%v' doesn't exist", params.Table)
	}

	query := fmt.Sprintf(`
			ALTER TABLE "%v"
				DROP CONSTRAINT pkey
		`, params.Table)

	_, err = transaction.Exec(query)
	if err != nil {
		return err
	}

	keys := ""
	for _, key := range table.PrimaryKeys {
		if key == ColumnName(params.Column) {
			continue
		}

		if keys == "" {
			keys = fmt.Sprintf(`"%v"`, key)
		} else {
			keys += fmt.Sprintf(`, "%v"`, key)
		}

	}

	query = fmt.Sprintf(`
		ALTER TABLE "%v"
			ADD CONSTRAINT pkey PRIMARY KEY (%v);
	`, params.Table, keys)

	_, err = transaction.Exec(query)
	if err != nil {
		return fmt.Errorf("can't add primary key '%v' to table '%v': %v/n", params.Column, params.Table, err)
	}

	return nil
}

func Sync() error {
	migrations, err := GetList()
	if err != nil {
		return fmt.Errorf("can't read migrations: %v/n", err)
	}

	dbConnectionString := fmt.Sprintf("user=%v password=%v dbname=%v host=%v port=%v sslmode=disable",
		"admin",
		"123456",
		"timeio",
		"localhost",
		5432)

	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		return fmt.Errorf("can't connect to db: %v", err)
	}
	defer func() { db.Close() }()

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("can't connect to db: %v", err)
	}

	log.Println("Connected to db")
	transaction, err := db.Begin()
	if err != nil {
		transaction.Rollback()
		return fmt.Errorf("can't start transaction: %v", err)
	}

	err = addMigrationsTableIfNotExist(transaction)
	if err != nil {
		transaction.Rollback()
		return fmt.Errorf("can't add migration table: %v", err)
	}

	currentMigrationId, err := getCurrentSyncedMigrationId(transaction)
	if err != nil {
		transaction.Rollback()
		return fmt.Errorf("can't read current migration state: %v", err)
	}

	_, err = GetCurrentSnapshot()
	if err != nil {
		return err
	}

	isCurrentMigrationPassed := currentMigrationId == ""

	for _, migration := range *migrations {

		if migration.Id == currentMigrationId {
			isCurrentMigrationPassed = true
			continue
		}

		if !isCurrentMigrationPassed {
			continue
		}

		err = applyMigrationActions(transaction, migration)
		if err != nil {
			transaction.Rollback()
			return fmt.Errorf("can't apply migration %v: %v/n", migration.Id, err)
		}

		addMigrationToMigrationsTable(transaction, migration)
		if err != nil {
			transaction.Rollback()
			return fmt.Errorf("can't add migration to migrations table %v: %v/n", migration.Id, err)
		}
	}

	return transaction.Commit()
}

func getCurrentSyncedMigrationId(transaction *sql.Tx) (string, error) {

	row := transaction.QueryRow("SELECT id FROM _migrations  ORDER BY id DESC  LIMIT 1")

	var migrationId string
	err := row.Scan(&migrationId)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return migrationId, err
}

func applyMigrationActions(transaction *sql.Tx, migration Migration) error {

	for index, action := range migration.Actions {
		var err error

		method, params, err := decodeAction(action.Method, action.Params)
		if err != nil {
			return fmt.Errorf("can't decode action %v/n", err)
		}

		switch method {
		case "addTable":
			err = applyAddTable(transaction, params.(AddTableParams))
			break
		case "deleteTable":
			err = applyDeleteTable(transaction, params.(DeleteTableParams))
			break
		case "addColumn":
			err = applyAddColumn(transaction, params.(AddColumnParams))
			break
		case "deleteColumn":
			err = applyDeleteColumn(transaction, params.(DeleteColumnParams))
			break
		case "addPrimaryKey":
			err = applyAddPrimaryKey(transaction, migration.Id, index, params.(AddPrimaryKeyParams))
			break
		case "deletePrimaryKey":
			err = applyDeletePrimaryKey(transaction, migration.Id, index, params.(DeletePrimaryKeyParams))
			break
		}

		if err != nil {
			return fmt.Errorf("can't apply action %v %v: %v/n", method, params, err)
		}
	}

	return nil
}

func decodeAction(method string, params json.RawMessage) (string, interface{}, error) {

	var err error
	switch method {
	case "addTable":
		var addTableParams AddTableParams
		err = json.Unmarshal(params, &addTableParams)
		if err != nil {
			return "", nil, err
		}

		return method, addTableParams, nil

	case "deleteTable":
		var deleteTableParams DeleteTableParams
		err = json.Unmarshal(params, &deleteTableParams)
		if err != nil {
			return "", nil, err
		}

		return method, deleteTableParams, nil

	case "addColumn":
		var addColumnParams AddColumnParams
		err = json.Unmarshal(params, &addColumnParams)
		if err != nil {
			return "", nil, err
		}

		return method, addColumnParams, nil

	case "deleteColumn":
		var deleteColumnParams DeleteColumnParams
		err = json.Unmarshal(params, &deleteColumnParams)
		if err != nil {
			return "", nil, err
		}

		return method, deleteColumnParams, nil

	case "addPrimaryKey":
		var addPrimaryKeyParams AddPrimaryKeyParams
		err = json.Unmarshal(params, &addPrimaryKeyParams)
		if err != nil {
			return "", nil, err
		}

		return method, addPrimaryKeyParams, nil

	case "deletePrimaryKey":
		var deletePrimaryKeyParams DeletePrimaryKeyParams
		err = json.Unmarshal(params, &deletePrimaryKeyParams)
		if err != nil {
			return "", nil, err
		}

		return method, deletePrimaryKeyParams, nil
	}

	return "", nil, nil
}

func addMigrationsTableIfNotExist(transaction *sql.Tx) error {
	_, err := transaction.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
        	id varchar(255) NOT NULL,
        	data text NOT NULL,
        	PRIMARY KEY (id)
    )`)

	return err
}

func addMigrationToMigrationsTable(transaction *sql.Tx, migration Migration) error {
	packedMigration, _ := json.Marshal(migration)
	_, err := transaction.Exec("INSERT INTO _migrations (id, data) VALUES ($1, $2)", migration.Id, packedMigration)
	return err
}