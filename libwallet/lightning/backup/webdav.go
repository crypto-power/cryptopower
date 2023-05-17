package backup

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/breez/breez/tor"
)

// A client represents a client connection to a {own|next}cloud
type WebdavClient struct {
	Url       *url.URL
	Username  string
	Password  string
	TorConfig *tor.TorConfig
}

type WebdavRequestError struct {
	StatusCode int
}

func (m *WebdavRequestError) Error() string {
	return fmt.Sprintf("%d", m.StatusCode)
}

// Error type encapsulates the returned error messages from the
// server.
type Error struct {
	// Exception contains the type of the exception returned by
	// the server.
	Exception string `xml:"exception"`

	// Message contains the error message string from the server.
	Message string `xml:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("Exception: %s, Message: %s", e.Exception, e.Message)
}

type ListFileResponse struct {
	XMLName xml.Name   `xml:"multistatus"`
	Files   []FileInfo `xml:"response"`
}

type FileInfo struct {
	XMLName xml.Name `xml:"response"`
	Href    string   `xml:"href"`
	Path    string
	Stat    FilePropStats `xml:"propstat"`
}

type FilePropStats struct {
	XMLName xml.Name   `xml:"propstat"`
	props   []FileProp `xml:"prop"`
}

type FileProp struct {
	XMLName      xml.Name `xml:"prop"`
	LastModified string   `xml:"getlastmodified"`
}

// Dial connects to an {own|next}Cloud instance at the specified
// address using the given credentials.
func Dial(host, username, password string) (*WebdavClient, error) {
	url, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	return &WebdavClient{
		Url:      url,
		Username: username,
		Password: password,
	}, nil
}

func toFolderPath(path string) string {
	if !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

// Mkdir creates a new directory on the cloud with the specified name.
func (c *WebdavClient) Mkdir(path string) error {
	_, err := c.sendWebDavRequest("MKCOL", toFolderPath(path), nil, nil)
	if webdavErr, ok := err.(*WebdavRequestError); ok {
		if webdavErr.StatusCode == 409 {
			return nil
		}
	}
	return err

}

// Delete removes the specified folder from the cloud.
func (c *WebdavClient) Delete(path string) error {
	_, err := c.sendWebDavRequest("DELETE", path, nil, nil)
	return err
}

// Upload uploads the specified source to the specified destination
// path on the cloud.
func (c *WebdavClient) Upload(src []byte, dest string) error {
	_, err := c.sendWebDavRequest("PUT", dest, src, nil)
	return err
}

// Download downloads a file from the specified path.
func (c *WebdavClient) Download(path string) ([]byte, error) {
	return c.sendWebDavRequest("GET", path, nil, nil)
}

func (c *WebdavClient) DirectoryExists(path string) bool {
	_, err := c.sendWebDavRequest("PROPFIND", toFolderPath(path), nil, map[string]string{
		"Depth": "0",
	})
	return err == nil
}

func (c *WebdavClient) ListDir(path string) (*ListFileResponse, error) {
	body, err := c.sendWebDavRequest("PROPFIND", toFolderPath(path), nil, map[string]string{
		"Depth": "1",
	})
	if err != nil {
		return nil, err
	}
	var response ListFileResponse
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	response.Files = response.Files[1:]

	for i, f := range response.Files {
		f.Path, err = c.getRelativePath(f.Href)
		if err != nil {
			return nil, err
		}
		response.Files[i] = f
	}
	return &response, err
}

func (c *WebdavClient) sendWebDavRequest(
	request string,
	path string,
	data []byte,
	headers map[string]string,
) ([]byte, error) {
	fmt.Printf("sendWebDavRequest %v: %v\n", request, path)

	var client *http.Client
	var err error
	if t := c.TorConfig; t != nil {
		fmt.Printf("sendWebDavRequest: proxying request using tor with config %+v.\n", *t)
		client, err = t.NewHttpClient()
		if err != nil {
			return nil, fmt.Errorf("sendWebDavRequest: unable to create tor http client: %w", err)
		}
	} else {
		client = &http.Client{}
	}
	joined := joinPath(c.Url.String(), path)
	req, err := http.NewRequest(request, joined, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/xml;charset=UTF-8")
	req.Header.Add("Accept", "application/xml,text/xml")
	req.Header.Add("Accept-Charset", "utf-8")
	req.Header.Add("Accept-Encoding", "")

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	req.SetBasicAuth(c.Username, c.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, &WebdavRequestError{StatusCode: resp.StatusCode}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		if body[0] == '<' {
			error := Error{}
			_ = xml.Unmarshal(body, &error)
			if error.Exception != "" {
				return nil, err
			}
		}
	}
	return body, nil
}

func (n *WebdavClient) getRelativePath(fileURL string) (string, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	absolutePath := n.Url.ResolveReference(u).String()
	serverPath := n.Url.String()
	relativePath := absolutePath[len(serverPath):]
	return relativePath, nil
}

func joinPath(path0 string, path1 string) string {
	return strings.TrimSuffix(path0, "/") + "/" + strings.TrimPrefix(path1, "/")
}
