package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/kubeshop/testkube/pkg/api/v1/testkube"
	"github.com/kubeshop/testkube/pkg/problem"
	"github.com/kubeshop/testkube/pkg/runner/output"
)

const (
	ClientHTTPTimeout = time.Minute
)

// check in compile time if interface is implemented
var _ Client = (*DirectScriptsAPI)(nil)

type Config struct {
	URI string `default:"http://localhost:8088"`
}

var config Config

func init() {
	envconfig.Process("TESTKUBE_API", &config)
}
func NewDirectScriptsAPI(uri string) DirectScriptsAPI {
	return DirectScriptsAPI{
		URI: uri,
		client: &http.Client{
			Timeout: ClientHTTPTimeout,
		},
	}
}

func NewDefaultDirectScriptsAPI() DirectScriptsAPI {
	return NewDirectScriptsAPI(config.URI)
}

type DirectScriptsAPI struct {
	URI    string
	client HTTPClient
}

// scripts and executions -----------------------------------------------------------------------------

func (c DirectScriptsAPI) GetScript(id string) (script testkube.Script, err error) {
	uri := c.getURI("/scripts/%s", id)
	resp, err := c.client.Get(uri)
	if err != nil {
		return script, err
	}

	if err := c.responseError(resp); err != nil {
		return script, fmt.Errorf("api/get-script returned error: %w", err)
	}

	return c.getScriptFromResponse(resp)
}

func (c DirectScriptsAPI) GetExecution(scriptID, executionID string) (execution testkube.Execution, err error) {
	uri := c.getURI("/scripts/%s/executions/%s", scriptID, executionID)

	resp, err := c.client.Get(uri)
	if err != nil {
		return execution, err
	}

	if err := c.responseError(resp); err != nil {
		return execution, fmt.Errorf("api/get-execution returned error: %w", err)
	}

	return c.getExecutionFromResponse(resp)
}

// ListExecutions list all executions for given script name
func (c DirectScriptsAPI) ListExecutions(scriptID string, limit int, tags []string) (executions testkube.ExecutionsResult, err error) {
	var uri string
	if len(tags) > 0 {
		uri = c.getURI("/scripts/%s/executions?pageSize=%d&tags=%s", scriptID, limit, strings.Join(tags, ","))
	} else {
		uri = c.getURI("/scripts/%s/executions?pageSize=%d", scriptID, limit)
	}

	resp, err := c.client.Get(uri)
	if err != nil {
		return executions, err
	}

	if err := c.responseError(resp); err != nil {
		return executions, fmt.Errorf("api/get-executions returned error: %w", err)
	}

	return c.getExecutionsFromResponse(resp)
}

func (c DirectScriptsAPI) DeleteScripts(namespace string) error {
	uri := c.getURI("/scripts?namespace=%s", namespace)
	return c.makeDeleteRequest(uri, true)
}

func (c DirectScriptsAPI) DeleteScript(name string, namespace string) error {
	if name == "" {
		return fmt.Errorf("script name '%s' is not valid", name)
	}
	uri := c.getURI("/scripts/%s?namespace=%s", name, namespace)
	return c.makeDeleteRequest(uri, true)
}

// CreateScript creates new Script Custom Resource
func (c DirectScriptsAPI) CreateScript(options UpsertScriptOptions) (script testkube.Script, err error) {
	uri := c.getURI("/scripts")

	request := testkube.ScriptUpsertRequest(options)

	body, err := json.Marshal(request)
	if err != nil {
		return script, err
	}

	resp, err := c.client.Post(uri, "application/json", bytes.NewReader(body))
	if err != nil {
		return script, err
	}

	if err := c.responseError(resp); err != nil {
		return script, fmt.Errorf("api/create-script returned error: %w", err)
	}

	return c.getScriptFromResponse(resp)
}

// UpdateScript creates new Script Custom Resource
func (c DirectScriptsAPI) UpdateScript(options UpsertScriptOptions) (script testkube.Script, err error) {
	uri := c.getURI("/scripts/%s", options.Name)
	request := testkube.ScriptUpsertRequest(options)

	body, err := json.Marshal(request)
	if err != nil {
		return script, err
	}

	req, err := http.NewRequest("PATCH", uri, bytes.NewReader(body))
	req.Header.Add("Content-type", "application/json")
	if err != nil {
		return script, fmt.Errorf("prepare request error: %w", err)
	}
	resp, err := c.client.Do(req)

	if err != nil {
		return script, err
	}

	if err := c.responseError(resp); err != nil {
		return script, fmt.Errorf("api/update-script returned error: %w", err)
	}

	return c.getScriptFromResponse(resp)
}

// ExecuteScript starts new external script execution, reads data and returns ID
// Execution is started asynchronously client can check later for results
func (c DirectScriptsAPI) ExecuteScript(id, namespace, executionName string, executionParams map[string]string) (execution testkube.Execution, err error) {
	// TODO call executor API - need to get parameters (what executor?) taken from CRD?
	uri := c.getURI("/scripts/%s/executions", id)

	// get script to get script tags
	script, err := c.GetScript(id)
	if err != nil {
		return execution, nil
	}

	request := testkube.ExecutionRequest{
		Name:      executionName,
		Namespace: namespace,
		Params:    executionParams,
		Tags:      script.Tags,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return execution, err
	}

	resp, err := c.client.Post(uri, "application/json", bytes.NewReader(body))
	if err != nil {
		return execution, err
	}

	if err := c.responseError(resp); err != nil {
		return execution, fmt.Errorf("api/execute-script returned error: %w", err)
	}

	return c.getExecutionFromResponse(resp)
}

// Logs reads logs from API SSE endpoint asynchronously
func (c DirectScriptsAPI) Logs(id string) (logs chan output.Output, err error) {
	logs = make(chan output.Output)
	uri := c.getURI("/executions/%s/logs", id)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return logs, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return logs, err
	}

	go func() {
		defer close(logs)
		defer resp.Body.Close()

		StreamToLogsChannel(resp.Body, logs)
	}()

	return
}

// ListScripts list all scripts in given namespace
func (c DirectScriptsAPI) ListScripts(namespace string, tags []string) (scripts testkube.Scripts, err error) {
	var uri string
	if len(tags) > 0 {
		uri = c.getURI("/scripts?namespace=%s&tags=%s", namespace, strings.Join(tags, ","))
	} else {
		uri = c.getURI("/scripts?namespace=%s", namespace)
	}

	resp, err := c.client.Get(uri)
	if err != nil {
		return scripts, fmt.Errorf("client.Get error: %w", err)
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return scripts, fmt.Errorf("api/list-scripts returned error: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&scripts)
	return
}

func (c DirectScriptsAPI) AbortExecution(scriptID, id string) error {
	uri := c.getURI("/scripts/%s/executions/%s", scriptID, id)
	err := c.makeDeleteRequest(uri, false)

	if err != nil {
		return fmt.Errorf("api/abort-script returned error: %w", err)
	}

	return nil
}

// executor --------------------------------------------------------------------------------

func (c DirectScriptsAPI) CreateExecutor(options CreateExecutorOptions) (executor testkube.ExecutorDetails, err error) {
	uri := c.getURI("/executors")

	request := testkube.ExecutorCreateRequest(options)

	body, err := json.Marshal(request)
	if err != nil {
		return executor, err
	}

	resp, err := c.client.Post(uri, "application/json", bytes.NewReader(body))
	if err != nil {
		return executor, err
	}

	if err := c.responseError(resp); err != nil {
		return executor, fmt.Errorf("api/create-executor returned error: %w", err)
	}

	return c.getExecutorDetailsFromResponse(resp)
}

func (c DirectScriptsAPI) GetExecutor(name string) (executor testkube.ExecutorDetails, err error) {
	uri := c.getURI("/executors/%s", name)
	resp, err := c.client.Get(uri)
	if err != nil {
		return executor, err
	}

	if err := c.responseError(resp); err != nil {
		return executor, fmt.Errorf("api/get-script returned error: %w", err)
	}

	return c.getExecutorDetailsFromResponse(resp)

}

func (c DirectScriptsAPI) ListExecutors() (executors testkube.ExecutorsDetails, err error) {
	uri := c.getURI("/executors?namespace=%s", "testkube")
	resp, err := c.client.Get(uri)
	if err != nil {
		return executors, fmt.Errorf("client.Get error: %w", err)
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return executors, fmt.Errorf("api/list-exeutors returned error: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&executors)
	return

}

func (c DirectScriptsAPI) DeleteExecutor(name string) (err error) {
	uri := c.getURI("/executors/%s?namespace=%s", name, "testkube")
	req, err := http.NewRequest("DELETE", uri, bytes.NewReader([]byte("")))
	if err != nil {
		return fmt.Errorf("prepare request error: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do error: %w", err)
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return fmt.Errorf("api/list-exeutors returned error: %w", err)
	}

	return
}

// maintenance --------------------------------------------------------------------------------------------

func (c DirectScriptsAPI) GetServerInfo() (info testkube.ServerInfo, err error) {
	uri := c.getURI("/info")
	resp, err := c.client.Get(uri)
	if err != nil {
		return info, err
	}

	err = json.NewDecoder(resp.Body).Decode(&info)

	return
}

// helper funcs --------------------------------------------------------------------------------

func (c DirectScriptsAPI) getExecutionFromResponse(resp *http.Response) (execution testkube.Execution, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&execution)
	return
}

func (c DirectScriptsAPI) getExecutionsFromResponse(resp *http.Response) (executions testkube.ExecutionsResult, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&executions)

	return
}

func (c DirectScriptsAPI) getScriptFromResponse(resp *http.Response) (script testkube.Script, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&script)
	return
}

func (c DirectScriptsAPI) getExecutorDetailsFromResponse(resp *http.Response) (executor testkube.ExecutorDetails, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&executor)
	return
}

func (c DirectScriptsAPI) getArtifactsFromResponse(resp *http.Response) (artifacts []testkube.Artifact, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&artifacts)

	return
}

func (c DirectScriptsAPI) responseError(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		var pr problem.Problem

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("can't get problem from api response: can't read response body %w", err)
		}
		defer resp.Body.Close()

		err = json.Unmarshal(bytes, &pr)
		if err != nil {
			return fmt.Errorf("can't get problem from api response: %w, output: %s", err, string(bytes))
		}

		return fmt.Errorf("problem: %+v", pr.Detail)
	}

	return nil
}

func (c DirectScriptsAPI) getURI(pathTemplate string, params ...interface{}) string {
	path := fmt.Sprintf(pathTemplate, params...)
	return fmt.Sprintf("%s/%s%s", c.URI, Version, path)
}

func (c DirectScriptsAPI) makeDeleteRequest(uri string, isContentExpected bool) error {
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return err
	}

	if isContentExpected && resp.StatusCode != http.StatusNoContent {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("request returned error: %s", respBody)
	}

	return nil
}

// GetExecutionArtifacts list all artifacts of the execution
func (c DirectScriptsAPI) GetExecutionArtifacts(executionID string) (artifacts testkube.Artifacts, err error) {
	uri := c.getURI("/executions/%s/artifacts", executionID)
	resp, err := c.client.Get(uri)
	if err != nil {
		return artifacts, err
	}

	if err := c.responseError(resp); err != nil {
		return artifacts, fmt.Errorf("api/list-artifacts returned error: %w", err)
	}

	return c.getArtifactsFromResponse(resp)
}

func (c DirectScriptsAPI) DownloadFile(executionID, fileName, destination string) (artifact string, err error) {
	uri := c.getURI("/executions/%s/artifacts/%s", executionID, url.QueryEscape(fileName))
	resp, err := c.client.Get(uri)
	if err != nil {
		return artifact, err
	}

	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return "", fmt.Errorf("error: %d", resp.StatusCode)
	}
	split := strings.Split(fileName, "/")
	f, err := os.Create(filepath.Join(destination, split[len(split)-1]))
	if err != nil {
		return artifact, err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return artifact, err
	}

	if err := c.responseError(resp); err != nil {
		return artifact, fmt.Errorf("api/download-file returned error: %w", err)
	}

	return f.Name(), nil
}

func (c DirectScriptsAPI) GetTest(id, namespace string) (script testkube.Test, err error) {
	uri := c.getURI("/tests/%s", id)
	resp, err := c.client.Get(uri)
	if err != nil {
		return script, err
	}

	if err := c.responseError(resp); err != nil {
		return script, fmt.Errorf("api/get-script returned error: %w", err)
	}

	return c.getTestFromResponse(resp)
}

// CreateTest creates new Test Custom Resource
func (c DirectScriptsAPI) CreateTest(options UpsertTestOptions) (script testkube.Test, err error) {
	uri := c.getURI("/tests")

	request := testkube.TestUpsertRequest(options)

	body, err := json.Marshal(request)
	if err != nil {
		return script, err
	}

	resp, err := c.client.Post(uri, "application/json", bytes.NewReader(body))
	if err != nil {
		return script, err
	}

	if err := c.responseError(resp); err != nil {
		return script, fmt.Errorf("api/create-test returned error: %w", err)
	}

	return c.getTestFromResponse(resp)
}

func (c DirectScriptsAPI) DeleteTest(name, namespace string) (err error) {
	uri := c.getURI("/tests/%s?namespace=%s", name, namespace)
	req, err := http.NewRequest("DELETE", uri, bytes.NewReader([]byte("")))
	if err != nil {
		return fmt.Errorf("prepare request error: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("client.Do error: %w", err)
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return fmt.Errorf("api/delete-test returned error: %w", err)
	}

	return
}

// UpdateTest creates new Test Custom Resource
func (c DirectScriptsAPI) UpdateTest(options UpsertTestOptions) (test testkube.Test, err error) {
	uri := c.getURI("/tests/%s", options.Name)

	request := testkube.TestUpsertRequest(options)

	body, err := json.Marshal(request)
	if err != nil {
		return test, err
	}

	req, err := http.NewRequest("PATCH", uri, bytes.NewReader(body))
	req.Header.Add("Content-type", "application/json")
	if err != nil {
		return test, fmt.Errorf("prepare request error: %w", err)
	}
	resp, err := c.client.Do(req)

	if err != nil {
		return test, err
	}

	if err := c.responseError(resp); err != nil {
		return test, fmt.Errorf("api/update-script returned error: %w", err)
	}

	return c.getTestFromResponse(resp)
}

// ListTests list all scripts in given namespace
func (c DirectScriptsAPI) ListTests(namespace string, tags []string) (tests testkube.Tests, err error) {
	var uri string
	if len(tags) > 0 {
		uri = c.getURI("/tests?namespace=%s&tags=%s", namespace, strings.Join(tags, ","))
	} else {
		uri = c.getURI("/tests?namespace=%s", namespace)
	}

	resp, err := c.client.Get(uri)
	if err != nil {
		return tests, fmt.Errorf("client.Get error: %w", err)
	}
	defer resp.Body.Close()

	if err := c.responseError(resp); err != nil {
		return tests, fmt.Errorf("api/list-tests returned error: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(&tests)

	return
}

// ExecuteTest starts new external test execution, reads data and returns ID
func (c DirectScriptsAPI) ExecuteTest(id, namespace, executionName string, executionParams map[string]string) (execution testkube.TestExecution, err error) {
	// TODO call executor API - need to get parameters (what executor?) taken from CRD?
	uri := c.getURI("/tests/%s/executions", id)

	request := testkube.TestExecutionRequest{
		Name:      executionName,
		Namespace: namespace,
		Params:    executionParams,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return execution, err
	}

	resp, err := c.client.Post(uri, "application/json", bytes.NewReader(body))
	if err != nil {
		return execution, err
	}

	if err := c.responseError(resp); err != nil {
		return execution, fmt.Errorf("api/execute-test returned error: %w", err)
	}

	return c.getTestExecutionFromResponse(resp)
}

func (c DirectScriptsAPI) GetTestExecution(executionID string) (execution testkube.TestExecution, err error) {
	uri := c.getURI("/test-executions/%s", executionID)

	resp, err := c.client.Get(uri)
	if err != nil {
		return execution, err
	}

	if err := c.responseError(resp); err != nil {
		return execution, fmt.Errorf("api/get-test-execution returned error: %w", err)
	}

	return c.getTestExecutionFromResponse(resp)
}

// ListExecutions list all executions for given script name
func (c DirectScriptsAPI) ListTestExecutions(testName string, limit int, tags []string) (executions testkube.TestExecutionsResult, err error) {
	var uri string
	if len(tags) > 0 {
		uri = c.getURI("/scripts/%s/executions?pageSize=%d&tags=%s", testName, limit, strings.Join(tags, ","))
	} else {
		uri = c.getURI("/scripts/%s/executions?pageSize=%d", testName, limit)
	}

	resp, err := c.client.Get(uri)

	if err != nil {
		return executions, err
	}

	if err := c.responseError(resp); err != nil {
		return executions, fmt.Errorf("api/list-test-executions returned error: %w", err)
	}

	return c.getTestExecutionsFromResponse(resp)
}

func (c DirectScriptsAPI) getTestFromResponse(resp *http.Response) (script testkube.Test, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&script)
	return
}

func (c DirectScriptsAPI) getTestExecutionFromResponse(resp *http.Response) (execution testkube.TestExecution, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&execution)
	return
}

func (c DirectScriptsAPI) getTestExecutionsFromResponse(resp *http.Response) (executions testkube.TestExecutionsResult, err error) {
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&executions)

	return
}
