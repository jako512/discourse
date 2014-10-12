package main

import (
	"io/ioutil"
	"testing"
)

var charmConfigBytes = []byte(`
{
	"DISCOURSE_DEVELOPER_EMAILS" : "foo@bar.com,baz@bat.com",
	"DISCOURSE_SMTP_ADDRESS" : "smtp.foo.com",
	"DISCOURSE_HOSTNAME" : "host.com",
	"DISCOURSE_SMTP_PORT" : 123,
	"DISCOURSE_SMTP_USER_NAME" : "user",
	"DISCOURSE_SMTP_PASSWORD" : "password",
	"UNICORN_WORKERS" : 3,
	"DISCOURSE_CDN_URL" : "cdn.com"
}`)

func TestParseCharmConfig(t *testing.T) {
	cfg, err := parseCharmConfig(charmConfigBytes)
	isNil(err, t)

	expected := charmConfig{
		DISCOURSE_DEVELOPER_EMAILS: sp("foo@bar.com,baz@bat.com"),
		DISCOURSE_SMTP_ADDRESS:     sp("smtp.foo.com"),
		DISCOURSE_HOSTNAME:         sp("host.com"),
		DISCOURSE_SMTP_PORT:        ip(123),
		DISCOURSE_SMTP_USER_NAME:   sp("user"),
		DISCOURSE_SMTP_PASSWORD:    sp("password"),
		UNICORN_WORKERS:            ip(3),
		DISCOURSE_CDN_URL:          sp("cdn.com"),
	}

	equals(expected, cfg, t)
}

func TestParseDiscourseConfig(t *testing.T) {
	b, err := ioutil.ReadFile("testing/standalone.yml")
	isNil(err, t)

	dc, err := parseDiscourseConfig(b)
	isNil(err, t)

	env, ok := dc["env"].(map[interface{}]interface{})
	equals(true, ok, t)
	equals(env["LANG"], "en_US.UTF-8", t)
	equals(env["DISCOURSE_DEVELOPER_EMAILS"], "me@example.com", t)
	equals(env["DISCOURSE_SMTP_ADDRESS"], "smtp.example.com", t)
	equals(env["UNICORN_WORKERS"], nil, t)
}

func TestMerge(t *testing.T) {
	b, err := ioutil.ReadFile("testing/standalone.yml")
	isNil(err, t)

	dc, err := parseDiscourseConfig(b)
	isNil(err, t)

	cfg, err := parseCharmConfig(charmConfigBytes)
	isNil(err, t)

	dc, err = merge(dc, cfg)
	isNil(err, t)

	env, ok := dc["env"].(map[interface{}]interface{})
	equals(true, ok, t)
	equals(env["LANG"], "en_US.UTF-8", t)
	equals(env["DISCOURSE_DEVELOPER_EMAILS"], "foo@bar.com,baz@bat.com", t)
	equals(env["DISCOURSE_HOSTNAME"], "host.com", t)
	equals(env["DISCOURSE_SMTP_ADDRESS"], "smtp.foo.com", t)
	equals(env["DISCOURSE_SMTP_PORT"], 123, t)
	equals(env["DISCOURSE_SMTP_USER_NAME"], "user", t)
	equals(env["DISCOURSE_SMTP_PASSWORD"], "password", t)
	equals(env["UNICORN_WORKERS"], 3, t)
	equals(env["DISCOURSE_CDN_URL"], "cdn.com", t)

}

func sp(s string) *string {
	return &s
}

func ip(i int) *int {
	return &i
}
