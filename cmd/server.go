package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"segmed-demo-back-end/internal"
	"strconv"
)

const MaxMultipartFormMemory = 32 << 20

func main() {
	// Setup the database connection and add all the reports to postgres and elastic search (if needed).
	internal.InitDb()

	// Defer closing the db connection until the main function exits.
	defer internal.CloseDb()

	router := gin.Default()
	router.Use(internal.HandleCorsMiddleware)
	router.MaxMultipartMemory = MaxMultipartFormMemory

	router.Static("/static", "web/static/")
	reportApiV1 := router.Group("/api/v1/report")
	{
		reportApiV1.GET("/getAllReportList", HandleGetAllReportList)
		reportApiV1.GET("/getReportList", HandleGetReportList)
		reportApiV1.GET("/getReport/:id", HandleGetReport)
		reportApiV1.GET("/getReportTags/:id", HandleGetReportTags)
		reportApiV1.GET("/getReportTagLists/:id", HandleGetReportTagLists)
		reportApiV1.PUT("/updateReportTags/:id", HandleUpdateReportTags)
	}

	log.Fatal(router.Run(":8081"))
}

func HandleGetAllReportList(c *gin.Context) {
	log.Println("HandleGetAllReportList attempting to get the information for all reports.")

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetAllReportList after trying to get all reports. The following "+
				"error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting report information."))

			return
		}
	}()

	reportList := internal.GetAllReportList()

	log.Println("Returning response with all report information.")
	c.JSON(http.StatusOK, reportList)

	return
}

func HandleGetReportList(c *gin.Context) {

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetReportList after trying to get report search results."+
				" The following error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("There was an error in getting report search results."))
			return
		}
	}()

	params := c.Request.URL.Query()
	searchTerm := params.Get("searchTerm")

	if searchTerm == "" {
		HandleGetAllReportList(c)
		return
	}

	log.Printf("HandleGetReportList attempting to get report search results for searchTerm = %s.\n",
		searchTerm)

	reportList := internal.GetReportListForSearchTerm(searchTerm)

	log.Println("Returning response with search results.")
	c.JSON(http.StatusOK, reportList)
	return
}

func HandleGetReport(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetReport after trying to get report. "+
				"The following error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting report."))
			return
		}
	}()

	log.Println("HandleGetReport attempting to get report.")

	idString := c.Param("id")
	id, err := strconv.ParseInt(idString, 10, 64)
	internal.HandleError(err)

	log.Printf("Getting report with id = %s.\n", id)

	report := internal.GetReportById(id)

	log.Println("Returning response with report.")

	c.JSON(http.StatusOK, report)
	return
}

func HandleGetReportTags(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetReportTags after trying to get all tags for report. "+
				"The following error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting report tags."))
			return
		}
	}()

	log.Println("HandleGetReportTags attempting to get all the tags for report.")

	idString := c.Param("id")
	id, err := strconv.ParseInt(idString, 10, 64)
	internal.HandleError(err)

	log.Printf("Getting tags for report with id = %s.\n", id)

	reportTagList := internal.GetReportTagListByReportId(id)

	log.Println("Returning response with tags for report.")

	c.JSON(http.StatusOK, reportTagList)
	return
}

func HandleUpdateReportTags(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleUpdateReportTags after trying to update tags for report. "+
				"The following error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting updating report tags."))
			return
		}
	}()

	log.Println("HandleUpdateReportTags attempting to update tags for report.")
	idString := c.Param("id")
	id, err := strconv.ParseInt(idString, 10, 64)
	internal.HandleError(err)

	reportExists := internal.DoesReportWithIdExist(id)
	if !reportExists {
		c.String(http.StatusBadRequest, fmt.Sprintf("No report found with id = %d.", id))
		return
	}

	jsonData, err := ioutil.ReadAll(c.Request.Body)
	internal.HandleError(err)

	var clientReportTagList []internal.ReportTag
	err = json.Unmarshal(jsonData, &clientReportTagList)
	internal.HandleError(err)

	internal.UpdateReportTagListByReportId(id, clientReportTagList)

	successMessage := "Successfully updated the reports tags."
	log.Println(successMessage)
	c.String(http.StatusOK, successMessage)
	return
}

// This function will return the active tags assigned to the report in one list, inactive tags assigned to to the
// report in another list, and tags not assigned to the report in another list.
func HandleGetReportTagLists(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in HandleGetReportTagLists after trying to get the active, "+
				"inactive, and unassigned tags for the report."+
				"The following error was encountered: ", r)

			c.String(http.StatusInternalServerError, fmt.Sprintf("Error in getting report tags."))
			return
		}
	}()

	idString := c.Param("id")
	id, err := strconv.ParseInt(idString, 10, 64)
	internal.HandleError(err)
	log.Printf("HandleGetReportTagLists attempting to get the active, inactive, and unassigned"+
		" tags for the report with id = %d.\n", id)

	reportTagList := internal.GetReportTagListByReportId(id)
	unassignedTagList := internal.GetUnassignedTagsByReportId(id)

	var activeReportTagList []internal.ReportTag
	var inactiveReportTagList []internal.ReportTag

	for _, reportTag := range reportTagList {
		if reportTag.Active {
			activeReportTagList = append(activeReportTagList, reportTag)
		} else {
			inactiveReportTagList = append(inactiveReportTagList, reportTag)
		}
	}

	jsonObject := make(map[string]interface{})
	jsonObject["activeTagList"] = activeReportTagList
	jsonObject["inactiveTagList"] = inactiveReportTagList
	jsonObject["unassignedTagList"] = unassignedTagList

	log.Println("Returning response with tags for report.")

	c.JSON(http.StatusOK, jsonObject)
	return
}
