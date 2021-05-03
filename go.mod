module github.com/argoproj/notifications-engine

go 1.16

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/RocketChat/Rocket.Chat.Go.SDK v0.0.0-20210112200207-10ab4d695d60
	github.com/antonmedv/expr v1.8.9
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/golang/mock v1.4.4
	github.com/google/go-github/v33 v33.0.0
	github.com/huandu/xstrings v1.3.0 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/opsgenie/opsgenie-go-sdk-v2 v1.0.5
	github.com/prometheus/client_golang v0.9.3
	github.com/sirupsen/logrus v1.6.0
	github.com/slack-go/slack v0.6.6
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.6.1
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	github.com/whilp/git-urls v0.0.0-20191001220047-6db9661140c0
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	gomodules.xyz/notify v0.1.0
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
)

// https://github.com/golang/go/issues/33546#issuecomment-519656923
replace github.com/go-check/check => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
