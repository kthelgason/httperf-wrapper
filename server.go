package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func handleServerConnection(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(64 << 20)
	file, _, err := r.FormFile("file")
	calls, _ := strconv.ParseUint(r.FormValue("calls"), 10, 64)
	defer file.Close()
	if err != nil {
		fmt.Println(err)
	}

	f, err := os.Create("/tmp/file")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	_, err = io.Copy(f, file)
	if err != nil {
		fmt.Println(err)
		return
	}
	enc := json.NewEncoder(w)
	res := runHTTPerf(5, calls, "/tmp/file")
	enc.Encode(res)
}

func parseResults(buf bytes.Buffer) map[string]string {
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	line, _ := buf.ReadBytes('\n')
	re := regexp.MustCompile("[a-z0-9 ]* requests ([0-9]+) replies ([0-9]+) test-duration ([0-9.]+) s")
	matches := re.FindAllSubmatch(line, 5)

	resp := map[string]string{}
	resp["requests"] = string(matches[0][1])
	resp["replies"] = string(matches[0][2])
	resp["test-duration"] = string(matches[0][3])

	buf.ReadBytes('\n')
	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Connection rate: ([0-9.]+) conn/s .*")
	matches = re.FindAllSubmatch(line, 2)

	resp["connection-rate"] = string(matches[0][1])

	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Request rate: ([0-9.]+) req/s .*")
	matches = re.FindAllSubmatch(line, 2)

	resp["request-rate"] = string(matches[0][1])

	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Reply time .ms.: response ([0-9.]+) transfer ([0-9.]+)")
	matches = re.FindAllSubmatch(line, 2)

	resp["response-time"] = string(matches[0][1])
	resp["transfer-time"] = string(matches[0][2])

	buf.ReadBytes('\n')
	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Reply status: 1xx=([0-9]+) 2xx=([0-9]+) 3xx=([0-9]+) 4xx=([0-9]+) 5xx=([0-9]+)")
	matches = re.FindAllSubmatch(line, 5)
	fmt.Printf("%s\n", matches)

	codes := make([]string, 5)

	for i := range codes {
		codes[i] = (string(matches[0][i+1]))
	}

	resp["status-codes"] = strings.Join(codes, ",")

	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	buf.ReadBytes('\n')
	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Errors: total ([0-9]+) client-timo ([0-9]+) socket-timo ([0-9]+) connrefused ([0-9]+) connreset ([0-9]+)")
	matches = re.FindAllSubmatch(line, 5)

	errors := make([]string, 9)

	for i := 0; i < 5; i++ {
		errors[i] = (string(matches[0][i+1]))
	}

	line, _ = buf.ReadBytes('\n')
	re = regexp.MustCompile("Errors: fd-unavail ([0-9]+) addrunavail ([0-9]+) ftab-full ([0-9]+) other ([0-9]+)")
	matches = re.FindAllSubmatch(line, 5)

	for i := 5; i < 9; i++ {
		errors[i] = (string(matches[0][i-4]))
	}

	resp["errors"] = strings.Join(errors, ",")

	return resp
}

func runHTTPerf(rate uint64, calls uint64, filename string) map[string]string {
	args := fmt.Sprintf("--hog --server=www.mbl.is --port=80 --num-calls=%d --wlog=y,%s", calls, filename)
	cmd := exec.Command("/usr/local/bin/httperf", strings.Fields(args)...)

	var out bytes.Buffer
	cmd.Stderr = os.Stdout
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}
	res := parseResults(out)
	return res

}

func main() {
	http.HandleFunc("/", handleServerConnection)
	http.ListenAndServe(":6666", nil)
}
