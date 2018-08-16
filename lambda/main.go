package main

// Based on https://github.com/aws-samples/lambda-go-samples
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Handler is your Lambda function handler
// It uses Amazon API Gateway request/responses provided by the aws-lambda-go/events package,
// However you could use other event sources (S3, Kinesis etc), or JSON-decoded primitive types such as 'string'.
func Handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("%s %s %s\n", req.HTTPMethod, req.Path, req.RequestContext.RequestID)

	switch req.HTTPMethod {
	case "POST":
		return rest_domains_post(req)
	case "PUT":
	case "GET":
	case "DELETE":
	}

	return events.APIGatewayProxyResponse{
		Body:       "error",
		StatusCode: 404,
	}, nil
}

// Handler to create a Sending Domain for a customer
func rest_domains_post(session events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if p := session.PathParameters; p == nil {
		jsonb, _ := json.Marshal("OK")

		// XXX: X-ENTITY-ID header
		return events.APIGatewayProxyResponse{
			Body:       string(jsonb),
			StatusCode: 200,
		}, nil
	} else {
		// XXX: verify on /{domain}/verify
		return events.APIGatewayProxyResponse{
			Body:       "error",
			StatusCode: 404,
		}, nil
	}
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
			err = json.Unmarshal(b, &req)

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
