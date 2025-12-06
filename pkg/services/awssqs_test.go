package services

import (
	"context"
	"fmt"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_AwsSqs(t *testing.T) {
	n := Notification{
		Message: "{{.message}}",
		AwsSqs: &AwsSqsNotification{
			MessageAttributes: map[string]string{
				"attributeKey": "{{.messageAttributeValue}}",
			},
			MessageGroupId: "{{.messageGroupId}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification

	err = templater(&notification, map[string]interface{}{
		"message":               "abcdef",
		"messageAttributeValue": "123456",
		"messageGroupId":        "a1b2c3",
	})

	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "abcdef", notification.Message)
	assert.Equal(t, map[string]string{
		"attributeKey": "123456",
	}, notification.AwsSqs.MessageAttributes)
	assert.Equal(t, "a1b2c3", notification.AwsSqs.MessageGroupId)
}

func TestSend_AwsSqs(t *testing.T) {
	// Overriding methods inside, so service.Send could be called.
	saveGetQueueURL := GetQueueURL
	saveSendMsg := SendMsg

	defer func() { SendMsg = saveSendMsg }()
	defer func() { GetQueueURL = saveGetQueueURL }()

	GetQueueURL = mockGetQueueURL("any", "")
	SendMsg = mockSendMsg("1", "")

	s := NewAwsSqsService(AwsSqsOptions{})

	destination := Destination{Recipient: "test"}
	notification := Notification{
		Message: "Hello",
		AwsSqs:  &AwsSqsNotification{},
	}

	if err := s.Send(notification, destination); err != nil {
		assert.NoError(t, err)
	}
}

func TestSendFail_AwsSqs(t *testing.T) {
	s := NewTypedAwsSqsService(AwsSqsOptions{
		Region: "us-east-1",
		AwsAccess: AwsAccess{
			Key:    "key",
			Secret: "secret",
		},
		EndpointUrl: "localhost",
		Account:     "123",
	})

	client := &fakeApi{"localhost", "1"}

	destination := Destination{Recipient: "test"}
	notification := Notification{
		Message: "Hello",
		AwsSqs:  &AwsSqsNotification{},
	}
	queueUrl, err := GetQueueURL(context.TODO(), client, s.getQueueInput(destination))
	assert.NoError(t, err)

	if _, err := SendMsg(context.TODO(), client, SendMessageInput(s, queueUrl.QueueUrl, notification)); err != nil {
		assert.Error(t, err)
	}
}

func TestGetConfigOptions_AwsSqs(t *testing.T) {
	s := NewTypedAwsSqsService(AwsSqsOptions{
		Region: "us-east-1",
		AwsAccess: AwsAccess{
			Key:    "key",
			Secret: "secret",
		},
		EndpointUrl: "localhost",
	})

	options := &config.LoadOptions{}
	optionsF := GetConfigOptions(s)

	for _, f := range optionsF {
		assert.NoError(t, f(options))
	}
	// Verify region properly set
	assert.Equal(t, "us-east-1", options.Region)
	// Get and Verify credentials from Provider
	creds, _ := options.Credentials.Retrieve(context.TODO())
	assert.Equal(t, s.opts.Key, creds.AccessKeyID)
	assert.Equal(t, s.opts.Secret, creds.SecretAccessKey)
}

func TestGetConfigOptionsFromEnv_AwsSqs(t *testing.T) {
	// Applying override via parameters instead of the ENV Variables
	finalKey, finalSecret, finalRegion := "key", "secret", "us-east-1"

	t.Setenv("AWS_ACCESS_KEY_ID", finalKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", finalSecret)
	t.Setenv("AWS_DEFAULT_REGION", finalRegion)

	s := NewTypedAwsSqsService(AwsSqsOptions{})

	options := GetConfigOptions(s)
	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	assert.NoError(t, err)

	creds, _ := cfg.Credentials.Retrieve(context.TODO())

	assert.Equal(t, finalKey, creds.AccessKeyID)
	assert.Equal(t, finalSecret, creds.SecretAccessKey)
	assert.Equal(t, finalRegion, cfg.Region)
}

func TestGetConfigOptionsOverrideCredentials_AwsSqs(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "env_key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env_secret")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-2")

	// Applying override via parameters instead of the ENV Variables
	finalKey, finalSecret, finalRegion := "key", "secret", "us-east-1"

	s := NewTypedAwsSqsService(AwsSqsOptions{
		Region: finalRegion,
		AwsAccess: AwsAccess{
			Key:    finalKey,
			Secret: finalSecret,
		},
	})

	options := GetConfigOptions(s)
	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	assert.NoError(t, err)

	creds, _ := cfg.Credentials.Retrieve(context.TODO())

	assert.Equal(t, finalKey, creds.AccessKeyID)
	assert.Equal(t, finalSecret, creds.SecretAccessKey)
	assert.Equal(t, finalRegion, cfg.Region)
}

func TestGetConfigOptionsCustomEndpointUrl_AwsSqs(t *testing.T) {
	// Will be overridden
	t.Setenv("AWS_DEFAULT_REGION", "us-east-2")

	finalKey, finalSecret, finalRegion, finalEndpoint := "key", "secret", "us-east-1", "localhost"

	s := NewTypedAwsSqsService(AwsSqsOptions{
		Region: finalRegion,
		AwsAccess: AwsAccess{
			Key:    finalKey,
			Secret: finalSecret,
		},
		EndpointUrl: finalEndpoint,
	})

	options := GetConfigOptions(s)
	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	assert.NoError(t, err)

	creds, _ := cfg.Credentials.Retrieve(context.TODO())

	assert.Equal(t, finalKey, creds.AccessKeyID)
	assert.Equal(t, finalSecret, creds.SecretAccessKey)
	assert.Equal(t, finalRegion, cfg.Region)
}

func TestGetClientOptionsCustomEndpointUrl_AwsSqs(t *testing.T) {
	// Will be overridden
	t.Setenv("AWS_DEFAULT_REGION", "us-east-2")

	finalKey, finalSecret, finalRegion, finalEndpoint := "key", "secret", "us-east-1", "localhost"

	s := NewTypedAwsSqsService(AwsSqsOptions{
		Region: finalRegion,
		AwsAccess: AwsAccess{
			Key:    finalKey,
			Secret: finalSecret,
		},
		EndpointUrl: finalEndpoint,
	})

	options := GetClientOptions(s)
	assert.Equal(t, 2, len(options))
}

func TestSendMessageInput_WithMessageGroupId_AwsSqs(t *testing.T) {
	s := NewTypedAwsSqsService(AwsSqsOptions{})
	queueUrl := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue.fifo"

	notification := Notification{
		Message: "Hello",
		AwsSqs: &AwsSqsNotification{
			MessageGroupId: "test-group-id",
		},
	}

	input := SendMessageInput(s, &queueUrl, notification)

	assert.Equal(t, &queueUrl, input.QueueUrl)
	assert.Equal(t, "Hello", *input.MessageBody)
	assert.Equal(t, int32(10), input.DelaySeconds)
	assert.Equal(t, "test-group-id", *input.MessageGroupId)
}

func TestSendMessageInput_WithoutMessageGroupId_AwsSqs(t *testing.T) {
	s := NewTypedAwsSqsService(AwsSqsOptions{})
	queueUrl := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"

	notification := Notification{
		Message: "Hello",
		AwsSqs: &AwsSqsNotification{
			MessageGroupId: "", // Empty string
		},
	}

	input := SendMessageInput(s, &queueUrl, notification)

	assert.Equal(t, &queueUrl, input.QueueUrl)
	assert.Equal(t, "Hello", *input.MessageBody)
	assert.Equal(t, int32(10), input.DelaySeconds)
	assert.Nil(t, input.MessageGroupId) // Should not be set
}

func TestSendMessageInput_WithoutAwsSqsNotification_AwsSqs(t *testing.T) {
	s := NewTypedAwsSqsService(AwsSqsOptions{})
	queueUrl := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"

	notification := Notification{
		Message: "Hello",
		AwsSqs:  nil, // No AWS SQS notification
	}

	input := SendMessageInput(s, &queueUrl, notification)

	assert.Equal(t, &queueUrl, input.QueueUrl)
	assert.Equal(t, "Hello", *input.MessageBody)
	assert.Equal(t, int32(10), input.DelaySeconds)
	assert.Nil(t, input.MessageGroupId) // Should not be set
}

// Helpers
var GetConfigOptions = (*awsSqsService).getConfigOptions
var GetClientOptions = (*awsSqsService).getClientOptions
var SendMessageInput = (*awsSqsService).sendMessageInput

var NewTypedAwsSqsService = func(opts AwsSqsOptions) *awsSqsService {
	return &awsSqsService{opts: opts}
}

type fakeApi struct {
	Url       string
	MessageId string
}

func (a fakeApi) SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	return &sqs.SendMessageOutput{
		MessageId: aws.String(a.MessageId),
	}, fmt.Errorf("%s", "fail scenario")
}

func (a fakeApi) GetQueueUrl(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	var err error

	return &sqs.GetQueueUrlOutput{
		QueueUrl: aws.String(a.Url),
	}, err
}

func mockSendMsg(messageId string, errorMsg string) func(c context.Context, api SQSSendMessageAPI, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return func(c context.Context, api SQSSendMessageAPI, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
		var err error
		if errorMsg != "" {
			err = fmt.Errorf("%s", errorMsg)
		}
		return &sqs.SendMessageOutput{
			MessageId: aws.String(messageId),
		}, err
	}
}

func mockGetQueueURL(queueUrl string, errorMsg string) func(c context.Context, api SQSSendMessageAPI, input *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return func(c context.Context, api SQSSendMessageAPI, input *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
		var err error
		if errorMsg != "" {
			err = fmt.Errorf("%s", errorMsg)
		}
		return &sqs.GetQueueUrlOutput{
			QueueUrl: aws.String(queueUrl),
		}, err
	}
}
