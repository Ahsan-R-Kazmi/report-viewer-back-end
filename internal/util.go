package internal

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

func HandleError(err error) {
	if err != nil {
		panic(err)
	}
}

// Allow all origins access, since the back-end application will not be accessible by the outside world.
// https://stackoverflow.com/questions/29418478/go-gin-framework-cors
func HandleCorsMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, "+
		"Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}

func ExtractValueFromLine(line string, key string) (value string) {
	value = ""
	if strings.Contains(line, key) {
		stringArray := strings.SplitN(line, ":", 2)
		value = stringArray[1]
		value = strings.TrimSpace(value)
	}

	return value
}

func DoHttpPutRequest(body []byte, url string) {
	log.Printf("DoHttpPutRequest attempting to make a PUT request to url %s.", url)
	client := &http.Client{}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))
	HandleError(err)

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)

	HandleError(err)
	log.Printf("Received response with Staus = %s.\n", resp.Status)
}

func DoHttpGetRequestWithBody(body []byte, url string) *http.Response {
	log.Printf("DoGetPutRequest attempting to make a GET request to url %s and body %s.", url, body)
	client := &http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(body))
	HandleError(err)

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)

	HandleError(err)
	log.Printf("Received response with Staus = %s.\n", resp.Status)

	return resp
}
