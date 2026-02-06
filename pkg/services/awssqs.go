package services

import (
	"bytes"
	"context"
	"os"
	texttemplate "text/template"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type AwsSqsNotification struct {
	MessageAttributes map[string]string `json:"messageAttributes"`
	MessageGroupId    string            `json:"messageGroupId,omitempty"`
}

type AwsSqsOptions struct {
	Queue       string `json:"queue"`
	Account     string `json:"account"`
	Region      string `json:"region"`
	EndpointUrl string `json:"endpointUrl,omitempty"`
	AwsAccess
}

type AwsAccess struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

func NewAwsSqsService(opts AwsSqsOptions) NotificationService {
	return &awsSqsService{opts: opts}
}

type awsSqsService struct {
	opts AwsSqsOptions
}

func (s awsSqsService) Send(notif Notification, dest Destination) error {
	cfgOptions := s.getConfigOptions()
	cfg, err := config.LoadDefaultConfig(context.TODO(), cfgOptions...)
	if err != nil {
		log.Fatalf("failed to load configuration, %v", err)
	}

	clientOptions := s.getClientOptions()
	client := sqs.NewFromConfig(cfg, clientOptions...)

	queueUrl, err := GetQueueURL(context.TODO(), client, s.getQueueInput(dest))
	if err != nil {
		log.Error("Got an error getting the queue URL: ", err)
		return err
	}

	sendMessage, err := SendMsg(context.TODO(), client, s.sendMessageInput(queueUrl.QueueUrl, notif))
	if err != nil {
		log.Error("Got an error sending the message: ", err)
		return err
	}
	log.Debug("Message Sent with Id: ", *sendMessage.MessageId)

	return nil
}

func (s awsSqsService) sendMessageInput(queueUrl *string, notif Notification) *sqs.SendMessageInput {
	input := &sqs.SendMessageInput{
		QueueUrl:     queueUrl,
		MessageBody:  aws.String(notif.Message),
		DelaySeconds: 10,
	}

	// Add MessageGroupId if available (required for FIFO queues)
	if notif.AwsSqs != nil && notif.AwsSqs.MessageGroupId != "" {
		input.MessageGroupId = aws.String(notif.AwsSqs.MessageGroupId)
	}

	return input
}

func (s awsSqsService) getQueueInput(dest Destination) *sqs.GetQueueUrlInput {
	result := &sqs.GetQueueUrlInput{}
	result.QueueName = &s.opts.Queue

	// Recipient in annotations takes precedent
	if dest.Recipient != "" {
		result.QueueName = &dest.Recipient
	}

	// Fill Account from configuration
	if s.opts.Account != "" {
		result.QueueOwnerAWSAccountId = &s.opts.Account
	}
	return result
}

func (s awsSqsService) getConfigOptions() []func(*config.LoadOptions) error {
	// Slice for AWS config options
	var options []func(*config.LoadOptions) error

	// When Credentials Are provided in service configuration - use them.
	if (s.opts != AwsSqsOptions{} && s.opts.Key != "" && s.opts.Secret != "") {
		// Use an empty session token when none is provided (previously used "default" which produced invalid session tokens)
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s.opts.Key, s.opts.Secret, "")))
	}

	// Fill Region from configuration
	if s.opts.Region != "" {
		options = append(options, config.WithRegion(s.opts.Region))
	}

	return options
}

func (s awsSqsService) getClientOptions() []func(*sqs.Options) {
	clientOptions := []func(o *sqs.Options){}
	if s.opts.EndpointUrl != "" {
		clientOptions = append(clientOptions, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(s.opts.EndpointUrl)
		})
	}
	endpointRegion := os.Getenv("AWS_DEFAULT_REGION")
	if s.opts.Region != "" {
		endpointRegion = s.opts.Region
	}
	if endpointRegion != "" {
		clientOptions = append(clientOptions, func(o *sqs.Options) {
			o.Region = endpointRegion
		})
	}
	return clientOptions
}

func (n *AwsSqsNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	groupId, err := texttemplate.New(name).Funcs(f).Parse(n.MessageGroupId)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]any) error {
		if notification.AwsSqs == nil {
			notification.AwsSqs = &AwsSqsNotification{}
		}

		if len(n.MessageAttributes) > 0 {
			notification.AwsSqs.MessageAttributes = n.MessageAttributes
			if err := notification.AwsSqs.parseMessageAttributes(name, f, vars); err != nil {
				return err
			}
		}

		var groupIdBuff bytes.Buffer
		if err := groupId.Execute(&groupIdBuff, vars); err != nil {
			return err
		}
		if val := groupIdBuff.String(); val != "" {
			notification.AwsSqs.MessageGroupId = val
		}

		return nil
	}, nil
}

func (n *AwsSqsNotification) parseMessageAttributes(name string, f texttemplate.FuncMap, vars map[string]any) error {
	for k, v := range n.MessageAttributes {
		var tempData bytes.Buffer

		tmpl, err := texttemplate.New(name).Funcs(f).Parse(v)
		if err != nil {
			continue
		}
		if err := tmpl.Execute(&tempData, vars); err != nil {
			return err
		}
		if val := tempData.String(); val != "" {
			n.MessageAttributes[k] = val
		}
	}
	return nil
}

type SQSSendMessageAPI interface {
	GetQueueUrl(ctx context.Context,
		params *sqs.GetQueueUrlInput,
		optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)

	SendMessage(ctx context.Context,
		params *sqs.SendMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

var GetQueueURL = func(c context.Context, api SQSSendMessageAPI, input *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return api.GetQueueUrl(c, input)
}

var SendMsg = func(c context.Context, api SQSSendMessageAPI, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return api.SendMessage(c, input)
}
