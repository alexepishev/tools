package vendors

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/devopsext/tools/common"
	"github.com/devopsext/utils"
)

//go:embed slack.tmpl
var msgTemplate string

const baseURL = "https://slack.com/api/"

const (
	filesUpload           = "files.upload"
	chatPostMessage       = "chat.postMessage"
	reactionsAdd          = "reactions.add"
	usersLookupByEmail    = "users.lookupByEmail"
	usergroupsUsersUpdate = "usergroups.users.update"
)

type SlackOptions struct {
	Timeout    int
	Insecure   bool
	Token      string
	Channel    string
	Title      string
	Message    string
	FileName   string
	File       string // content or path to file
	ImageURL   string
	ParentTS   string
	QuoteColor string
}

type SlackReactionOptions struct {
	Name string
}

type SlackOutputOptions struct {
	Output      string // path to output if empty to stdout
	OutputQuery string
}

type SlackMessage struct {
	Token       string
	Channel     string
	ParentTS    string
	Title       string
	Message     string
	ImageURL    string
	FileName    string
	FileContent string
	QuoteColor  string
}

type SlackUserEmail struct {
	Email string
}

type SlackUsergroupUsers struct {
	Usergroup string   `json:"usergroup"`
	Users     []string `json:"users"`
}

type Slack struct {
	client  *http.Client
	options SlackOptions
}

func (s *Slack) Send() ([]byte, error) {
	m := SlackMessage{
		Token:       s.options.Token,
		Channel:     s.options.Channel,
		Title:       s.options.Title,
		Message:     s.options.Message,
		FileContent: s.options.File,
		ParentTS:    s.options.ParentTS,
		QuoteColor:  s.options.QuoteColor,
	}
	return s.SendCustom(m)
}

func (s *Slack) SendFile() ([]byte, error) {
	m := SlackMessage{
		Token:       s.options.Token,
		Channel:     s.options.Channel,
		ParentTS:    s.options.ParentTS,
		Title:       s.options.Title,
		Message:     s.options.Message,
		ImageURL:    s.options.ImageURL,
		FileName:    s.options.FileName,
		FileContent: s.options.File,
	}
	return s.SendCustomFile(m)
}

func (s *Slack) SendMessage() ([]byte, error) {
	m := SlackMessage{
		Token:       s.options.Token,
		Channel:     s.options.Channel,
		ParentTS:    s.options.ParentTS,
		Title:       s.options.Title,
		Message:     s.options.Message,
		ImageURL:    s.options.ImageURL,
		FileName:    s.options.FileName,
		FileContent: s.options.File,
		QuoteColor:  s.options.QuoteColor,
	}
	return s.sendMessage(m)
}

func (s *Slack) SendCustomMessage(m SlackMessage) ([]byte, error) {
	return s.sendMessage(m)
}

func (s *Slack) sendMessage(m SlackMessage) ([]byte, error) {

	if m.Message == "" {
		return nil, errors.New("slack message is empty")
	}

	if m.Title == "" {
		// find the first nonempty line
		lines := strings.Split(m.Message, "\n")
		for _, line := range lines {
			if line != "" {
				m.Title = line
				break
			}
		}

		// if still empty, use the first line
		if m.Title == "" {
			m.Title = "No title"
		}

		m.Title = common.TruncateString(m.Title, 150)
	}
	jsonMsg, err := s.prepareMessage(m)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	b, err := s.post(m.Token, s.apiURL(chatPostMessage), q, "application/json; charset=utf-8", *jsonMsg)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Slack) SendCustom(m SlackMessage) ([]byte, error) {

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		w.Close()
	}()

	if err := w.WriteField("initial_comment", m.Message); err != nil {
		return nil, err
	}

	if err := w.WriteField("title", m.Title); err != nil {
		return nil, err
	}

	if err := w.WriteField("content", m.FileContent); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("channels", s.options.Channel)

	return s.post(m.Token, s.apiURL(filesUpload), q, w.FormDataContentType(), body)
}

func (s *Slack) SendCustomFile(m SlackMessage) ([]byte, error) {

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		w.Close()
	}()

	if !utils.IsEmpty(m.Message) {
		if err := w.WriteField("initial_comment", m.Message); err != nil {
			return nil, err
		}
	}

	if !utils.IsEmpty(m.Title) {
		if err := w.WriteField("title", m.Title); err != nil {
			return nil, err
		}
	}

	if !utils.IsEmpty(m.ParentTS) {
		if err := w.WriteField("thread_ts", m.ParentTS); err != nil {
			return nil, err
		}
	}

	fw, err := w.CreateFormFile("file", m.FileName)
	if err != nil {
		return nil, err
	}

	if _, err := fw.Write([]byte(m.FileContent)); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("channels", m.Channel)

	return s.post(m.Token, s.apiURL(filesUpload), q, w.FormDataContentType(), body)
}

func (s *Slack) prepareMessage(m SlackMessage) (*bytes.Buffer, error) {
	t, err := template.New("slack").Parse(msgTemplate)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	if err := t.Execute(b, m); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Slack) post(token string, URL string, query url.Values, contentType string, body bytes.Buffer) ([]byte, error) {

	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		return nil, err
	}

	if token == "" {
		token = s.options.Token
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	req.URL.RawQuery = query.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Slack) apiURL(cmd string) string {
	return baseURL + cmd
}

func (s *Slack) getAuth(opts SlackOptions) string {

	auth := ""
	if !utils.IsEmpty(opts.Token) {
		auth = fmt.Sprintf("Bearer %s", opts.Token)
		return auth
	}
	return auth
}

func (s *Slack) CustomAddReaction(slackOptions SlackOptions, reactionOptions SlackReactionOptions) ([]byte, error) {

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		w.Close()
	}()

	if err := w.WriteField("channel", slackOptions.Channel); err != nil {
		return nil, err
	}

	if err := w.WriteField("name", reactionOptions.Name); err != nil {
		return nil, err
	}

	if err := w.WriteField("timestamp", slackOptions.ParentTS); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return common.HttpPostRaw(s.client, s.apiURL(reactionsAdd), w.FormDataContentType(), s.getAuth(slackOptions), body.Bytes())
}

func (s *Slack) AddReaction(options SlackReactionOptions) ([]byte, error) {
	return s.CustomAddReaction(s.options, options)
}

func NewSlack(options SlackOptions) (*Slack, error) {

	slack := &Slack{
		client:  utils.NewHttpClient(options.Timeout, options.Insecure),
		options: options,
	}
	return slack, nil
}

func (s *Slack) CustomGetUser(slackOptions SlackOptions, slackUser SlackUserEmail) ([]byte, error) {
	params := make(url.Values)
	params.Add("email", slackUser.Email)

	u, err := url.Parse(s.apiURL(usersLookupByEmail))
	if err != nil {
		return nil, err
	}

	u.RawQuery = params.Encode()
	return common.HttpGetRaw(s.client, u.String(), "application/x-www-form-urlencoded", s.getAuth(slackOptions))

}

func (s *Slack) GetUser(options SlackUserEmail) ([]byte, error) {
	return s.CustomGetUser(s.options, options)
}

func (s *Slack) CustomUpdateUsergroup(slackOptions SlackOptions, slackUpdateUsergroup SlackUsergroupUsers) ([]byte, error) {

	body := &SlackUsergroupUsers{
		Usergroup: slackUpdateUsergroup.Usergroup,
		Users:     slackUpdateUsergroup.Users,
	}

	req, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return common.HttpPostRaw(s.client, s.apiURL(usergroupsUsersUpdate), "application/json", s.getAuth(slackOptions), req)
}

func (s *Slack) UpdateUsergroup(options SlackUsergroupUsers) ([]byte, error) {
	return s.CustomUpdateUsergroup(s.options, options)
}
