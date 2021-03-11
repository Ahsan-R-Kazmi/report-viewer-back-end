package main

import (
	"../internal"
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "postgres"
	dbPassword = "postgres"
	dbName     = "segmed"

	elasticSearchUrl = "http://localhost:9200"
	indexName = "segmed"
	typeName = "report"
)

type Report struct {
	Id int64		`json:"id"`
	FileName string `json:"fileName"`
	Name string 	`json:"name"`
	Author string	`json:"author"`
	Text string 	`json:"text"`
}
const MaxMultipartFormMemory = 32 << 20
const StaticTextFileLocation = "./web/static/text"

// Create a global reference to the db connection, so that it can be used in other functions.
// https://stackoverflow.com/questions/40587008/how-do-i-handle-opening-closing-db-connection-in-a-go-app/40587071
var db *sql.DB

func main() {

	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connectionString)
	internal.HandleError(err)

	// Defer closing the db connection until the main function exits.
	defer db.Close()

	router := gin.Default()
	router.Use(internal.HandleCorsMiddleware)
	router.MaxMultipartMemory = MaxMultipartFormMemory

	router.Static("/static", "web/static/")
	fileApiV1 := router.Group("/api/v1/file")
	{
		fileApiV1.GET("/getAllReportInfo", HandleGetAllReportInfo)
	}

	// On startup go through each of the static text files and see if they are already present in the report table.
	// If not create the corresponding record for the file in the report table and elasticsearch.
	AddAllReportsToDatabaseAndElasticSearch()

	log.Fatal(router.Run(":8081"))
}


func AddAllReportsToDatabaseAndElasticSearch() {
	files, err := ioutil.ReadDir(StaticTextFileLocation)
	internal.HandleError(err)

	for _, f := range files {
		AddReportToDatabaseAndElasticSearch(f.Name())
	}
}

func AddReportToDatabaseAndElasticSearch(fileName string) {
	log.Printf("Adding report with fileName = %s to database and elastic search.\n", fileName)

	file, err := os.Open(StaticTextFileLocation + "/" + fileName)
	if err != nil {
		internal.HandleError(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	counter := 0
	name := ""
	author := ""
	text := ""
	synopsis := ""

	for scanner.Scan() {
		line := scanner.Text()
		if counter < 5 {
			synopsis += line
		}

		if name == "" {
			name = internal.ExtractValueFromLine(line, "Title:")
		}

		if author == "" {
			author = internal.ExtractValueFromLine(line, "Author:")
		}

		text += line
		counter += 1
	}

	if err := scanner.Err(); err != nil {
		internal.HandleError(err)
	}

	// Check that a connection to the database can be opened.
	err = db.Ping()
	internal.HandleError(err)

	row := db.QueryRow("SELECT COUNT(*) FROM report WHERE file_name = $1", fileName)

	var count int64
	err = row.Scan(&count)
	internal.HandleError(err)

	if count == 0 {
		insertStatement := "INSERT INTO report(name, author, file_name, synopsis) VALUES($1, $2, $3, $4)"
		_, err = db.Exec(insertStatement, name, author, fileName, synopsis)
		internal.HandleError(err)
	}

	row = db.QueryRow("SELECT id FROM report WHERE file_name = $1", fileName)
	var id int64
	err = row.Scan(&id)
	internal.HandleError(err)

	report := Report{
		id,
		fileName,
		name,
		author,
		text,
	}

	jsonObject, err := json.Marshal(report)
	internal.HandleError(err)
	internal.DoHttpPutRequest(jsonObject, elasticSearchUrl + "/" + indexName + "/" + typeName + "/" + strconv.FormatInt(id, 10)  + "?pretty")
}


func HandleGetAllReportInfo(c *gin.Context) {
	log.Println("HandleGetAllReportInfo attempting to get the information for all reports.")

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetAllReportInfo after trying to get all reports. The following " +
				"error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting report information."))

			return
		}
	}()

	// Check that a connection to the database can be opened.
	err := db.Ping()
	internal.HandleError(err)

	rows, err := db.Query("SELECT id, name, author, file_name, synopsis FROM report ORDER by name ASC")
	internal.HandleError(err)

	defer rows.Close()

	var reportList []Report
	for rows.Next() {
		var id int64
		var name string
		var author string
		var fileName string
		var synopsis string
		err = rows.Scan(&id, &name, &author, &fileName, &synopsis)
		internal.HandleError(err)

		report := Report{
			id,
			name,
			author,
			fileName,
			synopsis,
		}

		reportList = append(reportList, report)
	}

	err = rows.Err()
	internal.HandleError(err)

	log.Println("Returning response with all report information.")
	c.JSON(http.StatusOK, reportList)

	return
}
