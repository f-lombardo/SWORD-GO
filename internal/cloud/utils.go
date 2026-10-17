package cloud

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func DecodeResponse(response *http.Response, out any) error {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, out); err != nil {
		return errors.New("API returned an invalid JSON response")
	}

	return nil
}
