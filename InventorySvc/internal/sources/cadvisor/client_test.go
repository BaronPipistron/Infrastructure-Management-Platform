package cadvisor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeContainersResponse_List(t *testing.T) {
	t.Parallel()

	body := []byte(`[
		{"name":"/","aliases":["root"]},
		{"name":"/docker/abc123","aliases":["abc123"],"namespace":"docker","spec":{"image":"nginx:1.27"}}
	]`)

	result, err := decodeContainersResponse(body)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "/docker/abc123", result[1].Name)
}

func TestDecodeContainersResponse_Map(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"/": {"aliases":["root"]},
		"/docker/abc123": {"aliases":["abc123"],"namespace":"docker"}
	}`)

	result, err := decodeContainersResponse(body)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "/", result[0].Name)
	require.Equal(t, "/docker/abc123", result[1].Name)
}

func TestDecodeContainersResponse_SingleObject(t *testing.T) {
	t.Parallel()

	body := []byte(`{"name":"/","aliases":["root"]}`)

	result, err := decodeContainersResponse(body)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "/", result[0].Name)
}

func TestDecodeContainersResponse_ErrorPayload(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"permission denied"}`)

	_, err := decodeContainersResponse(body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cadvisor api error")
}
