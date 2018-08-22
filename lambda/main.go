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
	"strings"
)

// Lambda function handler
func Handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Incoming: %s %s body %d bytes from SourceIP %s\n", req.HTTPMethod, req.Path, len(req.Body), req.RequestContext.Identity.SourceIP)

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
	Time   int                    `json:"time"`
	Host   string                 `json:"host"`
	Source string                 `json:"source"`
	Event  map[string]interface{} `json:"event"`
}

type splunkResult struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

// Handler for an incoming webhooks POST. Make outgoing request
func rest_webhooks_post(session events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Received Webhook data is an array [].msys.xxx_event_key.event
	var sparkPostWebhookEvents []map[string]map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(session.Body), &sparkPostWebhookEvents); err != nil {
		log.Fatal(err)
	}
	var splunkOutputLines string
	// Walk the contents, building a Splunk-compatible output
	for _, ev := range sparkPostWebhookEvents {
		// dereference msys.xxx_event_key, because "type" attribute is all we need to identify the event
		for _, event := range ev["msys"] {
			var se splunkEvent
			ts, err := strconv.Atoi(event["timestamp"].(string))
			if err != nil {
				log.Fatalf("Timestamp conversion error %s", err)
			}
			se.Time = ts
			se.Host = session.RequestContext.Identity.SourceIP
			se.Source = "SparkPost"
			se.Event = event
			jsonb, err := json.Marshal(se)
			if err != nil {
				log.Fatalf("JSON marshaling failed : %s", err)
			}
			splunkOutputLines += string(jsonb) + "\n"
		}
	}

	var buf = bytes.NewBufferString(splunkOutputLines)
	var splunkUrl = strings.Trim(os.Getenv("SPLUNK_URL"), " ") // Trim leading and trailing spaces, if present

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

	resBuffer := new(bytes.Buffer)
	resBuffer.ReadFrom(res.Body)
	resStr := resBuffer.String()
	// Special case: SparkPost sent an empty "ping" on webhook creation. Splunk returns Response 400 {"text":"No data","code":5}
	var splunkRes splunkResult
	if err := json.Unmarshal([]byte(resStr), &splunkRes); err != nil {
		log.Fatal(err)
	}
	if splunkOutputLines == "" && res.StatusCode >= 400 && splunkRes.Code == 5 && splunkRes.Text == "No data" {
		resStr = fmt.Sprintf("[response %d %s mapped -> 200 OK by this adapter]", res.StatusCode, resStr)
		res.StatusCode = 200
	}
	log.Printf("Outgoing: %s %s: Response %d %s", req.Method, splunkUrl, res.StatusCode, resStr)

	return events.APIGatewayProxyResponse{
		Body:       resStr,
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

			_, _ = Handler(req)
		}
	} else {
		// runtime code
		lambda.Start(Handler)
	}
}
