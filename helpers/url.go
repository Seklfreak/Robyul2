package helpers

import "net/url"

// UrlEncode encodes str with url#parse()
func UrlEncode(str string) (string, error) {
    u, err := url.Parse(str)

    if err != nil {
        return "", err
    }

    return u.String(), nil
}
