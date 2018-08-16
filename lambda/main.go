package main

// Based on https://github.com/aws-samples/lambda-go-samples
import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"bytes"
	"crypto/tls"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"net/http"
)

// Lambda function handler
func Handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("%s %s %s\n", req.HTTPMethod, req.Path, req.RequestContext.RequestID)

	switch req.HTTPMethod {
	case "POST":
		return rest_webhooks_post(req)
	default:
		return events.APIGatewayProxyResponse{
			Body:       "Unsupported method",
			StatusCode: 404,
		}, nil
	}
}

// Handler for an incoming webhooks POST. Make outgoing request
func rest_webhooks_post(session events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	const splunkUrl = "https://input-prd-p-52cm87k9p7sx.cloud.splunk.com:8088/services/collector"
	//const splunkUrl = "https://bigger-bin.herokuapp.com/1o8fwbd1"

	var buf = bytes.NewBufferString(session.Body)

	// Splunk provides x509: certificate signed by unknown authority :-( , so we need to skip those checks (like curl -v)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, _ := http.NewRequest("POST", splunkUrl, buf)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Splunk xyz")
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(res)
	buf2 := new(bytes.Buffer)
	buf2.ReadFrom(res.Body)
	return events.APIGatewayProxyResponse{
		Body:       buf2.String(),
		StatusCode: res.StatusCode,
	}, nil
}

func main() {
	if runtime.GOOS == "darwin" {
		// Simple code to simulate a request locally on OSX.  Takes a local JSON file as cmd-line arg
		if len(os.Args) < 2 {
			fmt.Println("Missing JSON filename in command line args")
		} else {
			requestFileName := os.Args[1]
			b, err := ioutil.ReadFile(requestFileName) // just pass the file name
			if err != nil {
				fmt.Println(err)
			}
			var req events.APIGatewayProxyRequest
			req.Body = string(b)
			req.HTTPMethod = "POST"

			var res events.APIGatewayProxyResponse
			res, _ = Handler(req)
			fmt.Println("Status code:\t", res.StatusCode)
			fmt.Println("Body:\t", res.Body)
			fmt.Println("Base64:\t", res.IsBase64Encoded)
		}
	} else {
		// runtime code
		lambda.Start(Handler)
	}
}
