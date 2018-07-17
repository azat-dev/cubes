package db

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const migrationsDirectoryName = "migrations"

type ColumnName string

type AddTableParams struct {
	Name string `json:"name"`
}

type DeleteTableParams struct {
	Name string `json:"name"`
}

type AddColumnParams struct {
	Table        string `json:"table"`
	Column       string `json:"column"`
	Type         string `json:"type"`
	IsNullable   bool   `json:"isNullable"`
	DefaultValue string `json:"defaultValue"`
}

type DeleteColumnParams struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

type AddPrimaryKeyParams struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

type DeletePrimaryKeyParams struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

type RenameColumnParams struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type Action struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type Migration struct {
	SchemaVersion string   `json:"schemaVersion"`
	Id            string   `json:"id"`
	Description   string   `json:"description"`
	Actions       []Action `json:"actions"`
}

func GetMigrationsDirectoryPath() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	directory := filepath.Join(pwd, migrationsDirectoryName)
	return directory, nil
}

func AddMigration(description string) error {

	id := time.Now().UTC().Format("20060102150405")
	fileName := id + ".json"
	migration := Migration{
		SchemaVersion: "1",
		Id:            id,
		Description:   description,
		Actions:       []Action{},
	}

	migrationsDir, err := GetMigrationsDirectoryPath()
	if err != nil {
		return err
	}

	//TODO: add checking usage of instance name
	if _, err := os.Stat(migrationsDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		err = os.Mkdir(migrationsDir, 0777)
		if err != nil {
			return err
		}
	}

	packedMigration, err := json.MarshalIndent(migration, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(migrationsDir, fileName), packedMigration, 0777)
}

func getMigrationPath(id string) (string, error) {

	migrationsDirectory, err := GetMigrationsDirectoryPath()
	if err != nil {
		return "", err
	}

	migrationPath := filepath.Join(migrationsDirectory, id+".json")
	return migrationPath, nil
}

func GetText(id string) (string, error) {

	migrationPath, err := getMigrationPath(id)
	if err != nil {
		return "", nil
	}

	migration, err := ioutil.ReadFile(migrationPath)
	return string(migration), nil
}

func Get(id string) (*Migration, error) {
	rawMigration, err := GetText(id)
	if err != nil {
		return nil, err
	}

	var migration Migration
	err = json.Unmarshal(([]byte)(rawMigration), &migration)

	if err != nil {
		return nil, fmt.Errorf("can't parse migration: %v/n", err)
	}

	return &migration, nil
}

func GetList() (*[]Migration, error) {

	migrationsDirectoryPath, err := GetMigrationsDirectoryPath()
	if err != nil {
		return nil, err
	}

	configsPathPattern := filepath.Join(migrationsDirectoryPath, "*.json")
	files, err := filepath.Glob(configsPathPattern)
	sort.Strings(files)

	if err != nil {
		return nil, err
	}

	result := []Migration{}

	for _, migrationPath := range files {
		_, fileName := filepath.Split(migrationPath)
		migrationId := strings.TrimSuffix(fileName, ".json")

		migration, err := Get(migrationId)
		if err != nil {
			return nil, fmt.Errorf("can't read migration %v/n", err)
		}

		result = append(result, *migration)
	}

	return &result, err
}

func addActionToMigrationFile(method string, params interface{}) (string, error) {

	migrations, err := GetList()
	if err != nil {
		return "", fmt.Errorf("can't get migration %v/n", err)
	}

	migrationsSize := len(*migrations)
	if migrationsSize == 0 {
		return "", fmt.Errorf("migration doesn't exist, please add migration/n")
	}

	_, err = GetSnapshotWithAction(method, params)
	if err != nil {
		return "", err
	}

	packedParams, _ := json.MarshalIndent(params, "", "  ")

	lastMigration := (*migrations)[migrationsSize-1]
	action := Action{
		Method: method,
		Params: (json.RawMessage)(packedParams),
	}

	lastMigration.Actions = append(lastMigration.Actions, action)

	packedMigration, _ := json.MarshalIndent(lastMigration, "", "  ")
	migrationPath, _ := getMigrationPath(lastMigration.Id)
	err = ioutil.WriteFile(migrationPath, packedMigration, 0777)
	if err != nil {
		return "", fmt.Errorf("can't write migration/n")
	}

	return lastMigration.Id, nil
}

func AddTable(tableName string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	params := AddTableParams{
		Name: tableName,
	}

	return addActionToMigrationFile("addTable", params)
}

func DeleteTable(tableName string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	params := DeleteTableParams{
		Name: tableName,
	}

	return addActionToMigrationFile("deleteTable", params)
}

func AddColumn(tableName string, columnName string, columnType string, isNullable bool, defaultValue string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	if strings.TrimSpace(columnName) == "" {
		return "", fmt.Errorf("column name is required /n")
	}

	if strings.TrimSpace(columnType) == "" {
		return "", fmt.Errorf("column type is required /n")
	}

	params := AddColumnParams{
		Table:        tableName,
		Column:       columnName,
		IsNullable:   isNullable,
		Type:         columnType,
		DefaultValue: defaultValue,
	}

	return addActionToMigrationFile("addColumn", params)
}

func DeleteColumn(tableName string, columnName string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	if strings.TrimSpace(columnName) == "" {
		return "", fmt.Errorf("column name is required /n")
	}

	params := DeleteColumnParams{
		Table:  tableName,
		Column: columnName,
	}

	return addActionToMigrationFile("deleteColumn", params)
}

func AddPrimaryKey(tableName string, columnName string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	if strings.TrimSpace(columnName) == "" {
		return "", fmt.Errorf("column name is required /n")
	}

	params := AddPrimaryKeyParams{
		Table:  tableName,
		Column: columnName,
	}

	return addActionToMigrationFile("addPrimaryKey", params)
}

func DeletePrimaryKey(tableName string, columnName string) (string, error) {

	if strings.TrimSpace(tableName) == "" {
		return "", fmt.Errorf("table name is required /n")
	}

	if strings.TrimSpace(columnName) == "" {
		return "", fmt.Errorf("column name is required /n")
	}

	params := DeletePrimaryKeyParams{
		Table:  tableName,
		Column: columnName,
	}

	return addActionToMigrationFile("deletePrimaryKey", params)
}