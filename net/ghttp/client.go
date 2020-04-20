package ghttp

/*
  其实fasthttp也有client的支持，但是不支持代理、验证等等

  参考资料
  使用socks5代理的demo http://mengqi.info/html/2015/201506062329-socks5-proxy-client-in-golang.html
  sosks4(a)代理的支持，可参考https://github.com/h12w/socks & https://github.com/reusee/httpc/blob/master/httpc.go


TODO: 从环境配置中读取并设置代理：https://stackoverflow.com/questions/51845690/how-to-program-go-to-use-a-proxy-when-using-a-custom-transport
*/

import (
	"context"
	"crypto/tls"
	"github.com/cavaliercoder/grab"
	"github.com/pkg/errors"
	"github.com/shawnwyckoff/gpkg/container/gjson"
	"github.com/shawnwyckoff/gpkg/container/gstring"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// url.Parse对于socks5:127.0.0.1:1234这种错误的url是无法判断错误的, 如果使用socks5:127.0.0.1:1234这种缺少//的格式，url.Parse将无法识别出端口号
func VerifyProxyFormat(s string) error {
	var allHTTPProxySchemes = []string{"http://", "https://", "socks5://"}

	hasScheme := func(s string) bool {
		for _, scheme := range allHTTPProxySchemes {
			if gstring.StartWith(s, scheme) {
				return true
			}
		}
		return false
	}
	if !hasScheme(s) {
		return errors.Errorf("invalid HTTP proxy address: %s", s)
	}
	_, err := url.Parse(s)
	if err != nil {
		return err
	}
	return nil
}

// Params:
// proxyAddr 支持http/https/socks5代理
//
// NOTICE
// 如果url不包含http://，将返回错误
// 如果followRedirect==false而且确实发生了跳转，则返回值的redirectUrl将被填写真实的跳转之后的URL；否则redirectUrl返回空
//
func Get(url string, proxy string, timeout time.Duration, followRedirect bool) (response *http.Response, err error) {
	hc := http.DefaultClient

	// redirects should be followed by default
	SetRedirect(hc, followRedirect)

	// Set proxy
	if proxy != "" {
		if err := SetProxy(hc, proxy); err != nil {
			return nil, err
		}
	}

	hc.Timeout = timeout

	return hc.Get(url)
}

func ReadBodyBytes(response *http.Response) ([]byte, error) {
	return ioutil.ReadAll(response.Body)
}

func ReadBodyString(response *http.Response) (string, error) {
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Your magic function. The Request in the Response is the last URL the
// client tried to access.
// 如果没有发生redirect，也能读到一个URL，只不过是原始请求的那个URL
func ReadFinalUrl(response *http.Response) (*url.URL, error) {
	if response == nil {
		return nil, errors.New("nil input")
	}
	// final URL
	return response.Request.URL, nil
}

// if you want auto guess filename
func GetBigFile(url string, filename string) (string, error) {
	if filename == "" {
		filename = "." // auto guess filename
	}
	resp, err := grab.Get(filename, url)
	if err != nil {
		return "", err
	}
	if filename == "." {
		filename = resp.Filename
	}
	return filename, nil
}

func GetBytes(url string, proxy string, timeout time.Duration) ([]byte, error) {
	resp, err := Get(url, proxy, timeout, true)
	if err != nil {
		return nil, err
	}
	return ReadBodyBytes(resp)
}

func GetString(url string, proxy string, timeout time.Duration) (string, error) {
	resp, err := Get(url, proxy, timeout, true)
	if err != nil {
		return "", err
	}
	return ReadBodyString(resp)
}

/*
func GetBytesWithProxies(urls []string, proxies []string, timeout time.Duration, maxRetry int) ([]bytes.Buffer, error) {
	r := []bytes.Buffer{}
	var rMu sync.Mutex
	var wg sync.WaitGroup


	for _, proxy := range proxies {
		wg.
		go func() {

		}()
		resp, _, err := Get(url, proxy, timeout, true)
		if err != nil {
			return nil, err
		}
		 ReadBodyBytes(resp)
	}
}*/

func CommonHttpMethods() []string {
	return []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}
}

func IsValidMethod(method string) bool {
	common := CommonHttpMethods()
	for _, v := range common {
		if v == method {
			return true
		}
	}
	return false
}

// Response is wrapper for standard http.Response and provides
// more methods.
type Resp struct {
	Response *http.Response
	Body     []byte
}

// String converts response body to string.
// An empty string will be returned if error.
func (r *Resp) String() string {
	return string(r.Body)
}

// newResponse creates new wrapper.
func newResp(r *http.Response) *Resp {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		body = []byte(`Error reading body:` + err.Error())
	}

	return &Resp{r, body}
}

// Do executes API request created by NewRequest method or custom *http.Request.
func DoRequest(req *http.Request, proxy string, timeout time.Duration, output ...interface{}) (*Resp, error) {
	client := http.DefaultClient
	if proxy != "" {
		if err := SetProxy(client, proxy); err != nil {
			return nil, err
		}
	}
	if timeout > 0 {
		SetTimeout(client, timeout)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	resp := newResp(response)
	if response.StatusCode == 200 && output != nil {
		for _, v := range output {
			if err := gjson.JSONDecode(resp.Body, v); err != nil {
				return nil, err
			}
		}
	}

	return resp, nil
}

// NewRequest create new HTTP request. Relative url can be provided in refURL.
func NewSimpleRequest(ctx context.Context, httpMethod string, baseURL, refURL string, params url.Values) (*http.Request, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}
	if params != nil {
		rel.RawQuery = params.Encode()
	}
	req, err := http.NewRequest(httpMethod, base.ResolveReference(rel).String(), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	return req, nil
}

func SetRedirect(client *http.Client, follow bool) {
	if follow { // redirects should be followed by default
		client.CheckRedirect = http.DefaultClient.CheckRedirect
	} else {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
}

func GetProxy(client *http.Client) string {
	if client == nil || client.Transport == nil {
		return ""
	}
	transport := client.Transport.(*http.Transport)
	req, _ := http.NewRequest("GET", "", nil)
	proxy, _ := transport.Proxy(req)
	return proxy.String()
}

func SetProxy(client *http.Client, proxyUrlString string) error {
	if err := VerifyProxyFormat(proxyUrlString); err != nil {
		return err
	}
	proxyURL, err := url.Parse(proxyUrlString) // url.Parse对于socks5:127.0.0.1:1234这种错误的url是无法判断错误的
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport)
	if client.Transport != nil {
		transport = client.Transport.(*http.Transport)
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	client.Transport = transport
	return nil
}

func SetProxy2(transport *http.Transport, proxyUrlString string) error {
	if transport == nil {
		return errors.Errorf("nil http.Transport")
	}
	if err := VerifyProxyFormat(proxyUrlString); err != nil {
		return err
	}
	proxyURL, err := url.Parse(proxyUrlString) // url.Parse对于socks5:127.0.0.1:1234这种错误的url是无法判断错误的
	if err != nil {
		return err
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	return nil
}

func SetInsecureSkipVerify(client *http.Client, skip bool) {
	transport := http.DefaultTransport.(*http.Transport)
	if client.Transport != nil {
		transport = client.Transport.(*http.Transport)
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = skip
	client.Transport = transport
}

func SetTimeout(client *http.Client, to time.Duration) {
	client.Timeout = to
}

func SetTLSTimeout(client *http.Client, tlsTimeout time.Duration) error {
	transport := http.DefaultTransport.(*http.Transport)
	if client.Transport != nil {
		transport = client.Transport.(*http.Transport)
	}
	transport.TLSHandshakeTimeout = tlsTimeout
	client.Transport = transport
	return nil
}