package p2p

import (
	"errors"

	"github.com/dghubble/sling"
	"github.com/kpango/glg"
)

var (
	s          = sling.New().Base(CentrumURL).Add("User-Agent", "Gizo Node")
	ErrNoToken = errors.New("Centrum: No token in struct")
)

type (
	DispatcherBody struct {
		Pub  string `url:"pub"`
		Ip   string `url:"ip"`
		Port int    `url:"port"`
	}

	Centrum struct {
		token string
	}
)

func NewCentrum() *Centrum {
	return new(Centrum)
}

func (c Centrum) GetToken() string {
	return c.token
}

func (c *Centrum) SetToken(token string) {
	c.token = token
}

func (c Centrum) GetDispatchers() map[string]interface{} {
	var dispatchers []string
	temp := make(map[string]interface{})
	_, err := s.New().Get("/v1/dispatchers").Receive(&dispatchers, &temp)
	if err != nil {
		glg.Fatal(err)
	}
	if len(dispatchers) != 0 {
		temp["dispatchers"] = dispatchers
	}
	return temp
}

func (c *Centrum) NewDisptcher(pub, ip string, port int) error {
	data := DispatcherBody{Pub: pub, Ip: ip, Port: port}
	res := make(map[string]interface{})
	_, err := s.Post("/v1/dispatcher").BodyForm(data).Receive(&res, &res)
	if err != nil {
		glg.Fatal(err)
	}
	token, ok := res["token"]
	if !ok {
		return errors.New(res["status"].(string))
	}
	c.SetToken(token.(string))
	return nil
}

func (c Centrum) ConnectWorker() (map[string]interface{}, error) {
	if c.GetToken() == "" {
		return nil, ErrNoToken
	}
	res := make(map[string]interface{})
	_, err := s.Patch("/v1/dispatcher/connect").Set("x-gizo-token", c.GetToken()).Receive(&res, &res)
	if err != nil {
		glg.Fatal(err)
	}
	return res, nil
}

func (c Centrum) DisconnectWorker() (map[string]interface{}, error) {
	if c.GetToken() == "" {
		return nil, ErrNoToken
	}
	res := make(map[string]interface{})
	_, err := s.Patch("/v1/dispatcher/disconnect").Set("x-gizo-token", c.GetToken()).Receive(&res, &res)
	if err != nil {
		glg.Fatal(err)
	}
	return res, nil
}

func (c Centrum) Wake() (map[string]interface{}, error) {
	if c.GetToken() == "" {
		return nil, ErrNoToken
	}
	res := make(map[string]interface{})
	_, err := s.Patch("/v1/dispatcher/wake").Set("x-gizo-token", c.GetToken()).Receive(&res, &res)
	if err != nil {
		glg.Fatal(err)
	}
	glg.Warn("Centrum: waking node")
	return res, nil
}

func (c Centrum) Sleep() (map[string]interface{}, error) {
	if c.GetToken() == "" {
		return nil, ErrNoToken
	}
	res := make(map[string]interface{})
	_, err := s.Patch("/v1/dispatcher/sleep").Set("x-gizo-token", c.GetToken()).Receive(&res, &res)
	if err != nil {
		glg.Fatal(err)
	}
	glg.Warn("Centrum: sleeping node")
	return res, nil
}
