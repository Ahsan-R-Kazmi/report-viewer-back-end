package internal

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

type Report struct {
	Id       int64  `json:"id"`
	FileName string `json:"fileName"`
	Name     string `json:"name"`
	Author   string `json:"author"`
	Synopsis string `json:"synopsis"`
	Text     string `json:"text"`
}

type EsReport struct {
	Report
	Score float64 `json:"score"`
}

type Tag struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ReportTag struct {
	Tag
	Active bool `json:"active"`
}

type Query struct {
	Query struct {
		Match struct {
			Text string `json:"text"`
		} `json:"match"`
	} `json:"query"`
}

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "postgres"
	dbPassword = "postgres"
	dbName     = "segmed"

	elasticSearchUrl = "http://localhost:9200"
	indexName        = "segmed"
	typeName         = "report"
)

const StaticTextFileLocation = "./web/static/text"

// Create a global reference to the db connection, so that it can be used in other functions.
// https://stackoverflow.com/questions/40587008/how-do-i-handle-opening-closing-db-connection-in-a-go-app/40587071
var db *sql.DB

func InitDb() {
	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connectionString)
	HandleError(err)

	// On startup go through each of the static text files and see if they are already present in the report table.
	// If not create the corresponding record for the file in the report table and elasticsearch.
	AddAllReportsToDatabaseAndElasticSearch()
}

func CloseDb() {
	err := db.Close()
	HandleError(err)
}

func AddAllReportsToDatabaseAndElasticSearch() {
	files, err := ioutil.ReadDir(StaticTextFileLocation)
	HandleError(err)

	for _, f := range files {
		AddReportToDatabaseAndElasticSearch(f.Name())
	}
}

func AddReportToDatabaseAndElasticSearch(fileName string) {
	log.Printf("Adding report with fileName = %s to database and elastic search.\n", fileName)

	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	row := db.QueryRow(`SELECT COUNT(*) FROM report WHERE file_name = $1`, fileName)

	var count int64
	err = row.Scan(&count)
	HandleError(err)

	if count != 0 {
		// Do not try to add the report again since it has already been added to the database (and elastic search).
		// TODO: Perhaps add a check that it is also in elastic search, and if not then add it.
		log.Printf("The report already exists, skipping further processing.\n")
		return
	}

	file, err := os.Open(StaticTextFileLocation + "/" + fileName)
	if err != nil {
		HandleError(err)
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
			synopsis += line + "\n"
		}

		if name == "" {
			name = ExtractValueFromLine(line, "Title:")
		}

		if author == "" {
			author = ExtractValueFromLine(line, "Author:")
		}

		text += line + "\n"
		counter += 1
	}

	if err := scanner.Err(); err != nil {
		HandleError(err)
	}

	if len(synopsis) > 0 {
		synopsis += "..."
	}

	// Add the report to the database.
	insertStatement := `INSERT INTO report(name , author, file_name, synopsis, text) VALUES($1, $2, $3, $4, $5)`
	_, err = db.Exec(insertStatement, name, author, fileName, synopsis, text)
	HandleError(err)

	row = db.QueryRow(`SELECT id FROM report WHERE file_name = $1`, fileName)
	var id int64
	err = row.Scan(&id)
	HandleError(err)

	report := Report{
		id,
		fileName,
		name,
		author,
		synopsis,
		text,
	}

	// Add the report to the elastic search.
	jsonData, err := json.Marshal(report)
	HandleError(err)
	DoHttpPutRequest(jsonData, elasticSearchUrl+"/"+indexName+"/"+typeName+"/"+strconv.FormatInt(id, 10)+"?pretty")
}

func GetReportById(id int64) Report {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	row := db.QueryRow(`SELECT id, file_name, name, author, synopsis, text FROM report WHERE id = $1;`, id)

	var fileName string
	var name string
	var author string
	var synopsis string
	var text string

	err = row.Scan(&id, &fileName, &name, &author, &synopsis, &text)
	HandleError(err)

	report := Report{
		id,
		fileName,
		name,
		author,
		synopsis,
		text,
	}

	return report
}

func GetAllReportList() []Report {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	rows, err := db.Query(`SELECT id, name, author, file_name, synopsis FROM report ORDER by name ASC`)
	HandleError(err)

	defer rows.Close()

	var reportList []Report
	for rows.Next() {
		var id int64
		var name string
		var author string
		var fileName string
		var synopsis string
		err := rows.Scan(&id, &name, &author, &fileName, &synopsis)
		HandleError(err)

		report := Report{
			id,
			fileName,
			name,
			author,
			synopsis,
			"", // The full text is not returned in this call for efficiency. It will be returned by the API for a single report.
		}

		reportList = append(reportList, report)
	}

	err = rows.Err()
	HandleError(err)

	return reportList
}

// If a searchTerm is provided, use elastic search since it can perform full-text search much more efficiently than
// a RDBMS.
func GetReportListForSearchTerm(searchTerm string) []EsReport {
	// Create the body for the elastic search query
	var query Query
	query.Query.Match.Text = searchTerm
	jsonData, err := json.Marshal(query)
	HandleError(err)

	resp := DoHttpGetRequestWithBody(jsonData, elasticSearchUrl+"/"+indexName+"/"+typeName+"/_search?pretty")

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	HandleError(err)

	var reportList []EsReport
	if resp.StatusCode == http.StatusOK {
		var jsonObject map[string]interface{}
		err := json.Unmarshal(bodyBytes, &jsonObject)
		HandleError(err)

		log.Printf(
			"There were %d hits; took: %dms\n",
			int(jsonObject["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)),
			int(jsonObject["took"].(float64)),
		)

		for _, hit := range jsonObject["hits"].(map[string]interface{})["hits"].([]interface{}) {
			source := hit.(map[string]interface{})["_source"]

			report := Report{
				int64(source.(map[string]interface{})["id"].(float64)),
				source.(map[string]interface{})["fileName"].(string),
				source.(map[string]interface{})["name"].(string),
				source.(map[string]interface{})["author"].(string),
				source.(map[string]interface{})["synopsis"].(string),
				"",
			}

			esReport := EsReport{
				Report: report,
				Score:  hit.(map[string]interface{})["_score"].(float64),
			}

			reportList = append(reportList, esReport)
		}

	} else {
		body := string(bodyBytes)
		errMsg := "error when making GET call to elastic search to get report search results"
		log.Printf(errMsg+": %s", body)
		HandleError(errors.New(errMsg))
	}

	return reportList
}

func GetReportTagListByReportId(id int64) []ReportTag {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	rows, err := db.Query(`SELECT t.id, t.name, t.color, rt.active FROM report AS r  
		INNER JOIN report_tag AS rt on r.id = rt.report_id
		INNER JOIN tag AS t ON rt.tag_id = t.id
		WHERE r.id = $1 ORDER BY t.name ASC;`, id)
	HandleError(err)

	defer rows.Close()

	var reportTagList []ReportTag
	for rows.Next() {
		var id int64
		var name string
		var color string
		var active bool

		err := rows.Scan(&id, &name, &color, &active)
		HandleError(err)

		tag := Tag{
			id,
			name,
			color,
		}

		reportTag := ReportTag{
			tag,
			active,
		}

		reportTagList = append(reportTagList, reportTag)
	}

	err = rows.Err()
	HandleError(err)

	return reportTagList
}

func DoesReportWithIdExist(id int64) bool {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	row := db.QueryRow(`SELECT COUNT(*) FROM report WHERE id = $1`, id)

	var count int64
	err = row.Scan(&count)
	HandleError(err)

	if count > 0 {
		return true
	}

	return false
}

func UpdateReportTagListByReportId(reportId int64, clientReportTagList []ReportTag) {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	clientReportTagMap := make(map[int64]ReportTag)
	for _, reportTag := range clientReportTagList {
		clientReportTagMap[reportTag.Id] = reportTag
	}

	// First get the serverReportTagList and convert the list to map with they key of the map the tag id.
	serverReportTagList := GetReportTagListByReportId(reportId)
	serverReportTagMap := make(map[int64]ReportTag)
	for _, reportTag := range serverReportTagList {
		serverReportTagMap[reportTag.Id] = reportTag
	}

	var reportTagsToUpdate []ReportTag
	var reportTagsToInsert []ReportTag
	var reportTagsToDelete []ReportTag

	// Then go through the clientReportTagList and see which values need to be updated. An update will need to occur
	// if a clientReportTag has corresponding serverReportTag with different active values.
	for _, clientReportTag := range clientReportTagList {
		if serverReportTag, exists := serverReportTagMap[clientReportTag.Id]; exists {

			// If the active values for the tags do not match, then update the server with the active values from the
			// client.
			if serverReportTag.Active != clientReportTag.Active {
				reportTagsToUpdate = append(reportTagsToUpdate, clientReportTag)
			}

			// Remove the tag from both maps, since it exists on both the client and server.
			delete(serverReportTagMap, clientReportTag.Id)
			delete(clientReportTagMap, clientReportTag.Id)
		}
	}

	// Insert any reportTags remaining in the clientReportTagMap, since they didn't have a corresponding tag on the server.
	for _, clientReportTag := range clientReportTagMap {
		reportTagsToInsert = append(reportTagsToInsert, clientReportTag)
	}

	// Delete any reportTags remaining in the serverReportTagMap, since they didn't have a corresponding tag on the client.
	for _, serverReportTag := range serverReportTagMap {
		reportTagsToDelete = append(reportTagsToDelete, serverReportTag)
	}

	for _, reportTag := range reportTagsToInsert {
		insertReportTag(reportId, reportTag)
	}

	for _, reportTag := range reportTagsToUpdate {
		updateReportTag(reportId, reportTag)
	}

	for _, reportTag := range reportTagsToDelete {
		deleteReportTag(reportId, reportTag)
	}

	return
}

func insertReportTag(reportId int64, tag ReportTag) {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	// Add the reportTag to the database.
	insertStatement := `INSERT INTO report_tag(report_id, tag_id, active) VALUES($1, $2, $3)`
	_, err = db.Exec(insertStatement, reportId, tag.Id, tag.Active)
	HandleError(err)
}

func deleteReportTag(reportId int64, tag ReportTag) {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	// Delete the reportTag from the database.
	deleteStatement := `DELETE FROM report_tag WHERE report_id = $1 AND tag_id = $2`
	_, err = db.Exec(deleteStatement, reportId, tag.Id)
	HandleError(err)
}

func updateReportTag(reportId int64, tag ReportTag) {
	// Check that a connection to the database can be opened.
	err := db.Ping()
	HandleError(err)

	// Update the reportTag in the database.
	updateStatement := `UPDATE report_tag SET active = $1 WHERE report_id = $2 AND tag_id = $3`
	_, err = db.Exec(updateStatement, tag.Active, reportId, tag.Id)
	HandleError(err)
}
