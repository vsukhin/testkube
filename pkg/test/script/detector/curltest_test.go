package detector

import (
	"testing"

	"github.com/kubeshop/testkube/pkg/api/v1/client"
	"github.com/stretchr/testify/assert"
)

const (
	curlValidContent       = `{ "command": [ "curl", "https://reqbin.com/echo/get/json", "-H", "'Accept: application/json'" ], "expected_status": 200, "expected_body": "{\"success\":\"true\"}" } `
	curlInvalidContent     = `{"some":"json content"}`
	curlInvalidJSONContent = `some non json content`
)

func TestCurlTestAdapter(t *testing.T) {

	t.Run("Is return true when valid content", func(t *testing.T) {
		detector := CurlTestAdapter{}
		name, is := detector.Is(client.UpsertScriptOptions{
			Content: curlValidContent,
		})

		assert.True(t, is, "content should be of curl/test type")
		assert.Equal(t, "curl/test", name)
	})

	t.Run("Is return false in case of invalid JSON content", func(t *testing.T) {
		detector := CurlTestAdapter{}
		name, is := detector.Is(client.UpsertScriptOptions{
			Content: curlInvalidContent,
		})

		assert.Empty(t, name)
		assert.False(t, is, "content should not be of curl/test type")

	})

	t.Run("Is return false in case of content which is not JSON ", func(t *testing.T) {
		detector := CurlTestAdapter{}
		name, is := detector.Is(client.UpsertScriptOptions{
			Content: curlInvalidJSONContent,
		})

		assert.Empty(t, name)
		assert.False(t, is, "content should not be of curl/test type")
	})
}
