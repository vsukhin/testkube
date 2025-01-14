/*
 * TestKube API
 *
 * TestKube provides a Kubernetes-native framework for test definition, execution and results
 *
 * API version: 1.0.0
 * Contact: testkube@kubeshop.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package testkube

// scripts create request body
type ScriptUpsertRequest struct {
	// script name - Custom Resource name - must be unique, use only lowercase numbers and dashes (-)
	Name string `json:"name,omitempty"`
	// script type - what executor type should be used during test execution
	Type_ string `json:"type,omitempty"`
	// kubernetes namespace (defaults to 'testkube')
	Namespace string `json:"namespace,omitempty"`
	// script content type can be:  - direct content - created from file,  - git repo directory checkout in case when test is some kind of project or have more than one file,
	InputType  string      `json:"inputType,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Repository *Repository `json:"repository,omitempty"`
	// script content - executor specific e.g. fo postman-collections executor
	Content string `json:"content,omitempty"`
}
