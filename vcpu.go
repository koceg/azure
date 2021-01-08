package main

// 20210108 this programm will obtain an Azure Token
// and query management.azure.com for quota status on used resources
// if resource is above 80% utilisation it will send a slack/mattermost message
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	form "net/url"
	"os"
	"strings"
)

type SP struct {
	appID    string
	secret   string
	tenentID string
	subscr   string
	region   string
	token    string
}

type Token struct {
	Token_type     string `json:"token_type"`
	Expires_in     string `json:"expires_in"`
	Ext_expires_in string `json:"ext_expires_in"`
	Expires_on     string `json:"expires_on"`
	Not_before     string `json:"not_before"`
	Resource       string `json:"resource"`
	Access_token   string `json:"access_token"`
}

type Limit struct {
	Limit        int    `json:"limit"`
	CurrentValue int    `json:"currentValue"`
	Unit         string `json:"unit"`
	Id           string `json:"id"`
	Name         *Name  `json:"name"`
}

type Name struct {
	Value          string `json:"value"`
	LocalizedValue string `json:"localizedValue"`
}

type Quota struct {
	Values []*Limit `json:"value"`
}

func main() {
	sp := new(SP)
	t := new(Token)
	spSet(sp)
	auth, err := oauth(sp)
	errorPrint(err)
	defer auth.Body.Close()
	body, err := ioutil.ReadAll(auth.Body)
	if err := json.Unmarshal(body, t); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	vcpu := "https://management.azure.com/subscriptions/%s/providers/Microsoft.Compute/locations/%s/usages?api-version=2020-06-01"
	storage := "https://management.azure.com/subscriptions/%s/providers/Microsoft.Storage/locations/%s/usages?api-version=2019-06-01"
	net := "https://management.azure.com/subscriptions/%s/providers/Microsoft.Network/locations/%s/usages?api-version=2020-07-01"
	getQuota(t.Access_token, sp.subscr, sp.region, vcpu)
	getQuota(t.Access_token, sp.subscr, sp.region, storage)
	getQuota(t.Access_token, sp.subscr, sp.region, net)
}

func spSet(sp *SP) {
	var ok bool
	if sp.appID, ok = os.LookupEnv("AZURE_CLIENT_ID"); !ok {
		errorPrint(fmt.Errorf("MIISSING AZURE_CLIENT_ID"))
	}
	if sp.secret, ok = os.LookupEnv("AZURE_CLIENT_SECRET"); !ok {
		errorPrint(fmt.Errorf("MIISSING AZURE_CLIENT_SECRET"))
	}
	if sp.tenentID, ok = os.LookupEnv("AZURE_TENANT_ID"); !ok {
		errorPrint(fmt.Errorf("MIISSING AZURE_TENANT_ID"))
	}
	if sp.subscr, ok = os.LookupEnv("AZURE_SUBSCRIPTION_ID"); !ok {
		errorPrint(fmt.Errorf("MIISSING AZURE_SUBSCRIPTION_ID"))
	}
	if sp.region, ok = os.LookupEnv("AZURE_REGION"); !ok {
		errorPrint(fmt.Errorf("MIISSING AZURE_REGION"))
	}
	if _, ok = os.LookupEnv("SLACK_WEBHOOK"); !ok {
		errorPrint(fmt.Errorf("MIISSING SLACK_WEBHOOK"))
	}
}

func oauth(s *SP) (*http.Response, error) {
	//https://docs.microsoft.com/en-us/rest/api/azure/
	url := "https://login.microsoftonline.com/%s/oauth2/token"
	c := new(http.Client)
	oauth := fmt.Sprintf(url, s.tenentID)
	data := form.Values{}
	data.Set("grant_type", "client_credentials")
	data.Add("client_id", s.appID)
	data.Add("client_secret", s.secret)
	data.Add("resource", "https://management.azure.com/")
	return c.PostForm(oauth, data)
}

func errorPrint(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getQuota(token, tenent, location, url string) {
	c := new(http.Client)
	q := new(Quota)
	rurl := fmt.Sprintf(url, tenent, location)
	req, err := http.NewRequest("GET", rurl, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	errorPrint(err)
	resp, err := c.Do(req)
	errorPrint(err)
	defer c.CloseIdleConnections()
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, q); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	checkQuota(q.Values)
}

func checkQuota(quota []*Limit) {
	for _, q := range quota {
		if float32(q.CurrentValue)/float32(q.Limit) > 0.80 {
			s := fmt.Sprintf(`{"text": "@channel Azure \"%s\" : %d/%d quota used"}`,
				q.Name.LocalizedValue, q.CurrentValue, q.Limit)
			r, err := sendMessage(s, os.Getenv("SLACK_WEBHOOK"))
			errorPrint(err)
			r.Body.Close()
			if r.StatusCode != 200 {
				errorPrint(fmt.Errorf("WEBHOOK HTTP ERROR: %s", r.Status))
			}
		}
	}
}

func sendMessage(quota, hook string) (*http.Response, error) {
	body := strings.NewReader(quota)
	req, err := http.NewRequest("POST", hook, body)
	errorPrint(err)
	req.Header.Set("Content-Type", "application/json")
	c := new(http.Client)
	return c.Do(req)
}
