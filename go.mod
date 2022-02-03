module github.com/argoproj/notifications-engine

go 1.16

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/RocketChat/Rocket.Chat.Go.SDK v0.0.0-20210112200207-10ab4d695d60
	github.com/antonmedv/expr v1.8.9
	github.com/bradleyfalzon/ghinstallation/v2 v2.0.4
	github.com/ghodss/yaml v1.0.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.4.0
	github.com/golang/mock v1.6.0
	github.com/google/go-github/v41 v41.0.0
	github.com/gregdel/pushover v1.1.0
	github.com/huandu/xstrings v1.3.0 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/opsgenie/opsgenie-go-sdk-v2 v1.0.5
	github.com/prometheus/client_golang v1.4.0
	github.com/sirupsen/logrus v1.6.0
	github.com/slack-go/slack v0.10.1
	github.com/spf13/cobra v1.3.0
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/whilp/git-urls v0.0.0-20191001220047-6db9661140c0
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	golang.org/x/tools v0.1.9 // indirect
	gomodules.xyz/notify v0.1.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
)

// https://github.com/golang/go/issues/33546#issuecomment-519656923
replace github.com/go-check/check => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
