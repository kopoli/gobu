// Generated with by licrep version v0.3.0
// https://github.com/kopoli/licrep
// Called with: licrep -o licenses.go

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
)

// License is a representation of an embedded license.
type License struct {
	// Name of the license
	Name string

	// The text of the license
	Text string
}

// GetLicenses gets a map of Licenses where the keys are
// the package names.
func GetLicenses() (map[string]License, error) {
	type EncodedLicense struct {
		Name string
		Text string
	}
	data := map[string]EncodedLicense{

		"github.com/kopoli/appkit": EncodedLicense{
			Name: "MIT",
			Text: `
H4sIAAAAAAAC/1xRzW7jNhC+8yk+5JQAQrrYY2+MRVtEJNKg6HV9pCU6YiuLhkg3yNsXIzu7zZ4Eceb7
HTt4NNKiDp2fksdjI+0TY6t4+ZjD25Dx2D3h+7fv3/DqxtHj1U3/uNkztvXzOaQU4oSQMPjZHz/wNrsp
+77AafYe8YRucPObL5Aj3PSBi59TnBCP2YUpTG9w6OLlg8UT8hASUjzldzd7uKmHSyl2wWXfo4/d9eyn
7DLpncLoEx7z4PHQ3hEPT4tI793IwgSafY7wHvIQrxmzT3kOHXEUCFM3Xnvy8DkewzncFQi+xE8sR1yT
LxafBc6xDyf6+iXW5XocQxoK9IGoj9fsCyR6XNosKMcfcUby48i6eAk+Ycn6y92yQ9YvVGi+V5To5X2I
569JQmKn6zyFNPgF00ekuCj+7btML7R+iuMY3ylaF6c+UKL0J2N0aneM//oly+26U8yhu9W9HODy66r3
URrcOOLo74X5HmGC+1+cmeRTdlMObsQlzove7zGfGbOVQKvXds+NgGyxNfqHLEWJB95Ctg8F9tJWemex
58ZwZQ/Qa3B1wKtUZQHx19aItoU2TDbbWoqygFSreldKtcHLzkJpi1o20ooSVoME71RStETWCLOquLL8
RdbSHgq2llYR51obcGy5sXK1q7nBdme2uhXgqoTSSqq1kWojGqHsM6SC0hA/hLJoK17XJMX4zlbakD+s
9PZg5KayqHRdCtPiRaCW/KUWNyl1wKrmsilQ8oZvxILSthKG0drNHfaVoCfS4wp8ZaVWFGOllTV8ZQtY
bexP6F62ogA3sqVC1kY3BaM69ZpWpCKcEjcWqhpfLqLN8r9rxU9ClILXUm1aAlPEz+Vn9l8AAAD//7MD
VDw4BAAA`,
		},
	}

	decode := func(input string) (string, error) {
		data := &bytes.Buffer{}
		br := base64.NewDecoder(base64.StdEncoding, strings.NewReader(input))

		r, err := gzip.NewReader(br)
		if err != nil {
			return "", err
		}

		_, err = io.Copy(data, r)
		if err != nil {
			return "", err
		}

		// Make sure the gzip is decoded successfully
		err = r.Close()
		if err != nil {
			return "", err
		}
		return data.String(), nil
	}

	ret := make(map[string]License)

	for k := range data {
		text, err := decode(data[k].Text)
		if err != nil {
			return nil, err
		}
		ret[k] = License{
			Name: data[k].Name,
			Text: text,
		}
	}

	return ret, nil
}
