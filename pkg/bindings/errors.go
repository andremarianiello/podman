package bindings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/blang/semver/v4"
	"github.com/containers/podman/v4/pkg/errorhandling"
)

var (
	ErrNotImplemented = errors.New("function not implemented")
)

func handleError(data []byte, unmarshalErrorInto interface{}) error {
	if err := json.Unmarshal(data, unmarshalErrorInto); err != nil {
		return fmt.Errorf("unmarshalling error into %#v, data %q: %w", unmarshalErrorInto, string(data), err)
	}
	return unmarshalErrorInto.(error)
}

// Process drains the response body, and processes the HTTP status code
// Note: Closing the response.Body is left to the caller
func (h APIResponse) Process(unmarshalInto interface{}) error {
	return h.ProcessWithError(unmarshalInto, &errorhandling.ErrorModel{})
}

// ProcessWithError drains the response body, and processes the HTTP status code
// Note: Closing the response.Body is left to the caller
func (h APIResponse) ProcessWithError(unmarshalInto interface{}, unmarshalErrorInto interface{}) error {
	data, err := io.ReadAll(h.Response.Body)
	if err != nil {
		return fmt.Errorf("unable to process API response: %w", err)
	}
	if h.IsSuccess() || h.IsRedirection() {
		if unmarshalInto != nil {
			if err := json.Unmarshal(data, unmarshalInto); err != nil {
				return fmt.Errorf("unmarshalling into %#v, data %q: %w", unmarshalInto, string(data), err)
			}
			return nil
		}
		return nil
	}

	if h.IsConflictError() {
		return handleError(data, unmarshalErrorInto)
	}

	// TODO should we add a debug here with the response code?
	return handleError(data, &errorhandling.ErrorModel{})
}

func CheckResponseCode(inError error) (int, error) {
	switch e := inError.(type) {
	case *errorhandling.ErrorModel:
		return e.Code(), nil
	case *errorhandling.PodConflictErrorModel:
		return e.Code(), nil
	default:
		return -1, errors.New("is not type ErrorModel")
	}
}

type APIVersionError struct {
	endpoint        string
	serverVersion   *semver.Version
	requiredVersion string
}

// NewAPIVersionError create bindings error when the endpoint on the server is not supported
// because the version is to old.
//   - endpoint is the name for the endpoint (e.g. /containers/json)
//   - version is the server API version
//   - requiredVersion is the server version need to use said endpoint
func NewAPIVersionError(endpoint string, version *semver.Version, requiredVersion string) *APIVersionError {
	return &APIVersionError{
		endpoint:        endpoint,
		serverVersion:   version,
		requiredVersion: requiredVersion,
	}
}

func (e *APIVersionError) Error() string {
	return fmt.Sprintf("API server version is %s, need at least %s to call %s", e.serverVersion.String(), e.requiredVersion, e.endpoint)
}
