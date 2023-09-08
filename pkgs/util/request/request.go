package request

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"go_pull/pkgs/vmconfig"
	"go_pull/pkgs/util/logtool"
	"net"
	"net/http"
	"time"

	//"fmt"
	//"os"
	"github.com/go-resty/resty/v2"
)

type reqr struct {
	Client  *resty.Client
	Clientr *resty.Request
	resp    *resty.Response
	Url     string
}

func (c *reqr) sethead(q string, a string) *reqr {
	c.Clientr.SetHeader(q, a)
	return c
}

func (c *reqr) Setheads(k map[string]string) *reqr {
	c.Clientr.SetHeaders(k)
	return c
}

func (c *reqr) Notparse() *reqr {
	c.Clientr.SetDoNotParseResponse(true)
	return c
}

func (c *reqr) Settls() *reqr {
	c.Client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	return c
}

func (c *reqr) Get() (*resty.Response, error) {
	return c.Clientr.Get(c.Url)
}

func (c *reqr) setresult(v *interface{}) *reqr {
	//var v interface{}
	c.Clientr.SetResult(v)
	return c
}

func (c *reqr) post(i ...string) (*resty.Response, error) {
	return c.Clientr.Post(c.Url)
}

//type Logger struct {
//}

//	func (e Logger) Errorf(format string, v ...interface{}) {
//		//	for _, e := range v {
//		//		switch e.(type) {
//		//		case *url.Error:
//		//			//fmt.Printf("%#v\v", e.(*url.Error))
//		//			switch b := e.(*url.Error).Err.(type) {
//		//			case *net.OpError:
//		//				{
//		//					fmt.Printf("%#v\n", b)
//		//				}
//		//			}
//		//			//fmt.Println(a, "(asd)")
//		//		}
//		//		//if errors.Is(e, url.Error) {
//		//		//	print(1)
//		//	}
//		//
//		//	fmt.Printf("%#v\n", v)
//
// }
//
// func (e Logger) Warnf(format string, v ...interface{}) {
// }
//
// func (e Logger) Debugf(format string, v ...interface{}) {
// }
func TimeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		//conn, err := net.DialTimeout(netw, addr, cTimeout)
		d := net.Dialer{
			Timeout:   cTimeout,
			DualStack: true}
		conn, err := d.Dial(netw, addr)

		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeout))
		return conn, nil
	}
}

func Requests(url string) *reqr {
	client := resty.New().
		SetTransport(&http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: true,
			Dial: TimeoutDialer(time.Duration(vmconfig.Ptimeout)*time.Second,
				time.Duration(vmconfig.Piotimeout)*time.Second),
		}).
		SetRetryCount(vmconfig.Retry).
		SetRetryWaitTime(100 * time.Nanosecond).
		AddRetryCondition(
			func(response *resty.Response, err error) bool {
				return !response.IsSuccess() || err != nil
			},
		).OnAfterResponse(
		func(c *resty.Client, resp *resty.Response) error {
			// Now you have access to Client and current Response object
			// manipulate it as per your need
			if !resp.IsSuccess() {
				if resp.StatusCode() == 401 {
					return nil
				}
				return errors.New("request failed,http code is " + resp.Status())

			}
			return nil // if its success otherwise return error
		},
	)

	//client.SetLogger(&Logger{})
	client.SetLogger(logtool.SugLog)

	return &reqr{Client: client,
		Clientr: client.R(),
		Url:     url}

	//
	//if err != nil {
	//fmt.Print(err)
	//	return err
	//  //log.Fatal(err)
	//}
	//return resp
	//fmt.Println("Response Info:",AuthSuccess)
	//fmt.Println("Status Code:", resp.StatusCode())
	//fmt.Println("Status:", resp.Status())
	//fmt.Println("Proto:", resp.Proto())
	//fmt.Println("Time:", resp.Time())
	//fmt.Println("Received At:", resp.ReceivedAt())
	//fmt.Println("Size:", resp.Size())
	//fmt.Println("Headers:")
	//for key, value := range resp.Header() {
	//  fmt.Println(key, "=", value)
	//}
	//fmt.Println("Cookies:")
	//for i, cookie := range resp.Cookies() {
	//  fmt.Printf("cookie%d: name:%s value:%s\n", i, cookie.Name, cookie.Value)
	//}
}

func Parsebody_to_json(resp *resty.Response) map[string]interface{} {
	//if err != nil{
	//	fmt.Println(err)
	//	os.Exit(1)
	//}
	var v interface{}
	json.Unmarshal(resp.Body(), &v)
	return v.(map[string]interface{})
}

func Ecocde_json(v any) ([]byte, error) {
	e, err := json.Marshal(v)
	return e, err
}

//func tojson(resp *resty.Response,err error) interface{} {
//	if err != nil{
//		fmt.Println(err)
//		os.Exit(1)
//	}
//	fmt.Println(resp.RawBody())
//	var v interface{}
//	json.NewDecoder(resp.RawBody()).Decode(&v)
//	return v
//}
