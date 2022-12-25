package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"

	//log "github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

type domainData struct {
	targetLink        string
	hostedDnsZoneId   string
	hostedDnsRecordId string
}

type HeClient struct {
	Username string
	Password string
	ApiKey   string
	HeUrl    string
	Method   string
	Client   *http.Client
}

func (hc *HeClient) AddTxtRecordWithLogin(ch *v1alpha1.ChallengeRequest) error {

	rn, domain, key := getNDK(ch)

	klog.InfoS("AddTxtRecordWithLogin", "rn", rn, "domain", domain, "key", key)

	body, err := hc.doLogin()
	if err != nil {
		return err
	}

	defer hc.doLogout()

	domainData, err := extractDomainData(body, domain)
	if err != nil {
		return err
	}

	//https://dns.he.net/?hosted_dns_zoneid=999999&menu=edit_zone&hosted_dns_editzone

	klog.InfoS("Creating the TXT record", "rn", rn, "domain", domain, "key", key, "domainData", domainData)

	postData := url.Values{}
	postData.Set("account", "")
	postData.Set("menu", "edit_zone")
	postData.Set("Type", "TXT")
	postData.Set("hosted_dns_zoneid", domainData.hostedDnsZoneId)
	postData.Set("hosted_dns_recordid", "")
	postData.Set("hosted_dns_editzone", "1")
	postData.Set("Priority", "")
	postData.Set("Name", rn)
	postData.Set("Content", key)
	postData.Set("TTL", "7200")
	postData.Set("hosted_dns_editrecord", "Submit")

	response, err := hc.Client.PostForm(hc.HeUrl+"index.cgi", postData)
	if err != nil {
		return fmt.Errorf("error creating record: %v", err)
	}

	body, err = readBody(response)
	if err != nil {
		return err
	}

	// check that the HTTP code is correct
	if response.StatusCode != 200 {
		return fmt.Errorf("got invalid status code %v", response.StatusCode)
	}

	// check that we're on the right page: there should be a ">Successfully added new record to {domain}<" message
	// or, if the record already exists, a ">Insert failed.  Unable to update.  That record already exists." message
	msg1 := fmt.Sprintf(">Successfully added new record to %v<", domain)
	msg2 := ">Insert failed.  Unable to update.  That record already exists."
	if !(strings.Contains(body, msg1) || strings.Contains(body, msg2)) {
		return fmt.Errorf("cannot find the expected creation message in page")
	}

	klog.InfoS("Successfully created record")
	return nil

}

func (hc *HeClient) RemoveTxtRecordWithLogin(ch *v1alpha1.ChallengeRequest) error {

	rn, domain, key := getNDK(ch)

	klog.InfoS("RemoveTxtRecordWithLogin", "rn", rn, "domain", domain, "key", key)

	body, err := hc.doLogin()
	if err != nil {
		return err
	}
	defer hc.doLogout()

	domainData, err := extractDomainData(body, domain)
	if err != nil {
		return err
	}

	//https://dns.he.net/?hosted_dns_zoneid=999999&menu=edit_zone&hosted_dns_editzone

	newUrl := hc.HeUrl + domainData.targetLink

	// we have to actually go there to get the record id
	response, err := hc.Client.Get(newUrl)
	if err != nil {
		return err
	}
	body, err = readBody(response)
	if err != nil {
		return err
	}

	// check that we're in the right page
	wantedMsg := fmt.Sprintf(">Managing zone: %s<", domain)
	if !strings.Contains(body, wantedMsg) {
		return fmt.Errorf("cannot find the 'managing zone' message in page")
	}

	x, err := extractRecordId(body, rn, domain, key)
	if err != nil {
		return err
	}
	domainData.hostedDnsRecordId = x

	klog.InfoS("Deleting the TXT record", "rn", rn, "domain", domain, "key", key, "domainData", domainData)

	postData := url.Values{}
	postData.Set("hosted_dns_zoneid", domainData.hostedDnsZoneId)
	postData.Set("hosted_dns_recordid", domainData.hostedDnsRecordId)
	postData.Set("menu", "edit_zone")
	postData.Set("hosted_dns_delconfirm", "delete")
	postData.Set("hosted_dns_editzone", "1")
	postData.Set("hosted_dns_delrecord", "1")

	response, err = hc.Client.PostForm(hc.HeUrl+"index.cgi", postData)
	if err != nil {
		return fmt.Errorf("error deleting record: %v", err)
	}

	body, err = readBody(response)
	if err != nil {
		return err
	}

	// check that the HTTP code is correct
	if response.StatusCode != 200 {
		return fmt.Errorf("got invalid status code %v", response.StatusCode)
	}

	// check that we're on the right page: there should be a ">Successfully removed record.<" message
	wantedMsg = ">Successfully removed record.<"
	if !strings.Contains(body, wantedMsg) {
		return fmt.Errorf("cannot find the successful deletion message in page")
	}

	klog.InfoS("Successfully deleted record")

	return nil

}

func (hc *HeClient) AddTxtRecordWithDynamicDns(ch *v1alpha1.ChallengeRequest) error {

	rn, domain, key := getNDK(ch)

	klog.InfoS("AddTxtRecordWithDynamicDns", "rn", rn, "domain", domain, "key", key)

	//curl "https://dyn.dns.he.net/nic/update" -d "hostname=_acme-challenge.solartis.it" -d 'password=mychallenge' -d "txt=FOOBAR"

	postData := url.Values{}
	postData.Set("hostname", rn+"."+domain)
	postData.Set("password", hc.ApiKey)
	postData.Set("txt", key)

	response, err := hc.Client.PostForm(hc.HeUrl+"nic/update", postData)
	if err != nil {
		return fmt.Errorf("submission error: %v", err)
	}

	// to be successful, the response should start with either "good " or "nochg "
	// status code 200

	klog.V(4).InfoS("Dynamic DNS response", "status", response.Status, "headers", response.Header)

	body, err := readBody(response)

	if err != nil {
		return err
	}

	if !(body[:5] == "good " || body[:6] == "nochg ") {
		return fmt.Errorf("submission failed, response body is '%v'", body)
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected response status %v", response.StatusCode)
	}

	klog.InfoS("Successfully added record")
	return nil

}

func (hc *HeClient) RemoveTxtRecordWithDynamicDns(ch *v1alpha1.ChallengeRequest) error {

	rn, domain, key := getNDK(ch)

	klog.InfoS("RemoveTxtRecordWithDynamicDns", "rn", rn, "domain", domain, "key", key)

	//curl "https://dyn.dns.he.net/nic/update" -d "hostname=_acme-challenge.solartis.it" -d 'password=mychallenge' -d "txt=FOOBAR"

	// we just overwrite the TXT with a dummy value;
	// we could even do nothing at all, for that matter
	postData := url.Values{}
	postData.Set("hostname", rn+"."+domain)
	postData.Set("password", hc.ApiKey)
	postData.Set("txt", "UNUSED")

	response, err := hc.Client.PostForm(hc.HeUrl+"nic/update", postData)
	if err != nil {
		return fmt.Errorf("submission error: %v", err)
	}

	// to be successful, the response should start with either "good " or "nochg "
	// status code 200

	klog.V(4).InfoS("Dynamic DNS response", "status", response.Status, "headers", response.Header)

	body, err := readBody(response)

	if err != nil {
		return err
	}

	if !(body[:5] == "good " || body[:6] == "nochg ") {
		err = fmt.Errorf("submission failed, response body is '%v'", body)
		return err
	}

	if response.StatusCode != 200 {
		err = fmt.Errorf("unexpected response status %v", response.StatusCode)
		return err
	}

	klog.InfoS("Successfully deleted record")
	return nil
}

// extract the record name, the domain, and the key from the request
func getNDK(ch *v1alpha1.ChallengeRequest) (string, string, string) {

	// Strip the zone from the fqdn to yield the record name
	rn := strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone)
	rn = strings.TrimSuffix(rn, ".") // Also remove any stray .

	// Remove trailing . from domain
	domain := strings.TrimSuffix(ch.ResolvedZone, ".")

	return rn, domain, ch.Key
}

// find the HE record ID from a page
func extractRecordId(body string, rn string, domain string, key string) (string, error) {

	klog.V(4).InfoS("extractRecordId looking for key", "key", key)

	tree, err := htmlquery.Parse(strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("error parsing HTML body: %v", err)
	}

	/*
			Typical tr structure (slightly reformatted):

			<tr class="dns_tr" id="4819821031" title="Click to edit this item." onclick="editRow(this)">
				<td class="hidden">933183</td>
				<td class="hidden">4819821031</td>
				<td width="95%" class="dns_view">mytxt.mydomain.com</td>

				<!-- <td align="center" ><img src="/include/images/types/txt.gif" data="TXT" alt="TXT"/></td> -->
				<td align="center" ><span class="rrlabel TXT" data="TXT" alt="TXT" >TXT</span></td>
				<td align="left">7200</td>
				<td align="center">-</td>
				<td align="left" data="&quot;CONTENTS&quot;" onclick="event.cancelBubble=true; alert($(this).attr('data'));" title="Click to view entire contents." >&quot;CONTENTS&quot;</td>
				<td class="hidden">0</td>
				<td></td>
				<td align="center" class="dns_delete"  onclick="event.cancelBubble=true;deleteRecord('4819821031','mytxt.domain.com','TXT')" title="Click to delete this record.">
				<img src="/include/images/delete.png" alt="delete"/>
				</td>
		</tr>
	*/

	// NOTE: the "tbody" isn't in the actual html, but since go's parser adds it,
	// we must include it in the xpath
	for _, tr := range htmlquery.Find(tree, "//div[@id='dns_main_content']/table/tbody/tr[@class='dns_tr']") {

		td := htmlquery.Find(tr, "./td[4]/span")[0]
		recordType := htmlquery.SelectAttr(td, "data")

		klog.V(4).InfoS("Reading record", "recordType", recordType)

		if recordType != "TXT" {
			continue
		}

		td = htmlquery.Find(tr, "./td[7]")[0]

		// apparently this does unescaping too
		txtValue := htmlquery.SelectAttr(td, "data")
		// remove quotes
		txtValue = strings.Trim(txtValue, "\"")

		td = htmlquery.Find(tr, "./td[@class='dns_delete']")[0]
		onclick := htmlquery.SelectAttr(td, "onclick")
		m := regexp.MustCompile(`^event\.cancelBubble=true;deleteRecord\(\s*'([^']*)'\s*,\s*'([^']*)'\s*,\s*'([^']*)'\s*\)$`)
		res := m.FindAllStringSubmatch(onclick, -1)

		recordId, recordName, recordType := res[0][1], res[0][2], res[0][3]

		klog.V(4).InfoS("Parsed record info", "txtValue", txtValue, "recordId", recordId, "recordName", recordName, "recordType", recordType)

		if !(recordName == rn+"."+domain && recordType == "TXT" && txtValue == key) {
			continue
		}

		// found
		return recordId, nil
	}

	return "", fmt.Errorf("cannot find record to remove in zone")

}

func extractDomainData(body string, domain string) (*domainData, error) {

	klog.V(4).InfoS("extractDomainData", "domain", domain)

	tree, err := htmlquery.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error parsing response body: %v", err)
	}

	// look for wanted domains
	targetLink := ""
	hostedDnsZoneId := ""
	for _, tr := range htmlquery.Find(tree, "//table[@id='domains_table']/tbody/tr") {
		d := htmlquery.InnerText(htmlquery.Find(tr, "./td[3]/span")[0])
		if d != domain {
			continue
		}
		href := htmlquery.SelectAttr(htmlquery.Find(tr, "./td[2]/img")[0], "onclick")
		m := regexp.MustCompile(`^javascript:document\.location\.href='(.*)'$`)
		targetLink = m.ReplaceAllString(href, "$1")
		m = regexp.MustCompile(`.*hosted_dns_zoneid=(\d+).*`)
		hostedDnsZoneId = m.ReplaceAllString(targetLink, "$1")
		break
	}

	if targetLink == "" {
		return nil, fmt.Errorf("requested domain %v not found", domain)
	}

	return &domainData{
		targetLink:      targetLink,
		hostedDnsZoneId: hostedDnsZoneId,
	}, nil
}

func (hc *HeClient) doLogout() error {
	// fetch initial page to get the cookie
	klog.InfoS("Logging out...")
	_, err := hc.Client.Get(hc.HeUrl + "?action=logout")
	return err
}

func readBody(response *http.Response) (string, error) {

	b, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if err != nil {
		return "", fmt.Errorf("read response error: %v", err)
	}
	return string(b), nil

}

func (hc *HeClient) doLogin() (string, error) {

	if hc.Username == "" || hc.Password == "" {
		return "", fmt.Errorf("empty username or password")
	}

	// fetch initial page to get the cookie
	klog.InfoS("Fetching initial page", "url", hc.HeUrl)
	_, err := hc.Client.Get(hc.HeUrl)

	if err != nil {
		return "", fmt.Errorf("error fetching initial page '%v': %v", hc.HeUrl, err)
	}

	klog.InfoS("Logging in", "username", hc.Username)
	postData := url.Values{}
	postData.Set("email", hc.Username)
	postData.Set("pass", hc.Password)
	postData.Set("submit", "Login!")

	response, err := hc.Client.PostForm(hc.HeUrl, postData)
	if err != nil {
		return "", fmt.Errorf("login error: %v", err)
	}

	klog.V(4).InfoS("Login response", "status", response.Status, "headers", response.Header)

	body, err := readBody(response)

	if err != nil {
		return "", err
	}

	if strings.Contains(body, ">Incorrect</div>") {
		err = fmt.Errorf("login failed (invalid credentials?)")
		return "", err
	}

	return body, nil
}
