package main

// Based on https://github.com/aws-samples/lambda-go-samples
import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strconv"

	"bytes"
	"crypto/tls"
	"encoding/json"
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

// For marshalling Splunk events
type splunkEvent struct {
	time   int                    `json:"time"`
	host   string                 `json:"host"`
	source string                 `json:"source"`
	event  map[string]interface{} `json:"event"`
}

// Handler for an incoming webhooks POST. Make outgoing request
func rest_webhooks_post(session events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//const splunkUrl = "https://input-prd-p-52cm87k9p7sx.cloud.splunk.com:8088/services/collector"
	const splunkUrl = "https://bigger-bin.herokuapp.com/1btyn7r1"

	var buf = bytes.NewBufferString(session.Body)
	// Received Webhook data is an array [].msys.xxx_event_key.event
	var sparkPostWebhookEvents []map[string]map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(session.Body), &sparkPostWebhookEvents); err != nil {
		log.Fatal(err)
	}
	// Walk the contents, building a Splunk-compatible output
	for _, ev := range sparkPostWebhookEvents {
		// dereference msys.xxx_event_key, because "type" attribute is all we need to identify the event
		for _, event := range ev["msys"] {
			var se splunkEvent
			ts, err := strconv.Atoi(event["timestamp"].(string))
			if err != nil {
				log.Fatalf("Timestamp conversion error %s", err)
			}
			se.time = ts
			se.host = "SparkPost" //TODO: replace with session IP adddr
			se.source = "SparkPost"
			se.event = event
			jsonb, err := json.Marshal(se)
			if err != nil {
				log.Fatalf("JSON marshaling failed : %s", err)
			}
			fmt.Println(string(jsonb))
		}
	}

	// Splunk provides x509: certificate signed by unknown authority :-( , so we need to skip those checks (like curl -k)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	// Selectively copy across request method and headers

	req, _ := http.NewRequest(session.HTTPMethod, splunkUrl, buf)
	for hname, hval := range session.Headers {
		switch hname {
		case "Accept-Encoding", "Accept", "Authorization", "Content-Type":
			req.Header.Add(hname, hval)
		case "User-Agent":
			req.Header.Add(hname, "SparkPost-Splunk adapter")
		}
	}
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
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
			err = json.Unmarshal(b, &req)

			var res events.APIGatewayProxyResponse
			res, _ = Handler(req)
			fmt.Println("Status code:\t", res.StatusCode)
			fmt.Println("Body:\t", res.Body)
		}
	} else {
		// runtime code
		lambda.Start(Handler)
	}
}
