package services

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/notifications-engine/pkg/util/text"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type AwsSqsNotification struct {
	Body string `json:"body"`
}

type AwsSqsOptions struct {
	Queue   string `json:"queue"`
	Account string `json:"account"`
	Region  string `json:"region"`
	AwsAccess
}

type AwsAccess struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

func NewAwsSqsService(opts AwsSqsOptions) NotificationService {
	return &awsSqservice{opts: opts}
}

type awsSqservice struct {
	opts AwsSqsOptions
}

func (s awsSqservice) Send(notification Notification, dest Destination) error {
	// If body provided inside of the template merge it with required message.
	if notification.AwsSqs != nil {
		notification.Message = text.Coalesce(notification.AwsSqs.Body, notification.Message)
	}

	// Slice for AWS config options
	var options []func(*config.LoadOptions) error

	// When Credentials Are provided in service configuration use them.
	if (s.opts != AwsSqsOptions{} && s.opts.AwsAccess.Key != "" && s.opts.AwsAccess.Secret != "") {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s.opts.AwsAccess.Key, s.opts.AwsAccess.Secret, "default")))
	}

	// Fill Region from configuration
	if s.opts.Region != "" {
		options = append(options, config.WithRegion(s.opts.Region))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)

	if err != nil {
		log.Fatalf("failed to load configuration, %v", err)
	}

	client := sqs.NewFromConfig(cfg)

	// Get URL of queue
	gQInput := &sqs.GetQueueUrlInput{}

	gQInput.QueueName = &s.opts.Queue

	// destination in annotation takes precedent
	if dest.Recipient != "" {
		gQInput.QueueName = &dest.Recipient
	}

	// Fill Account from configuration
	if s.opts.Account != "" {
		gQInput.QueueOwnerAWSAccountId = &s.opts.Account
	}

	result, err := GetQueueURL(context.TODO(), client, gQInput)
	if err != nil {
		log.Error("Got an error getting the queue URL: ", err)
		return err
	}

	queueURL := result.QueueUrl

	sMInput := &sqs.SendMessageInput{
		QueueUrl:     queueURL,
		MessageBody:  aws.String(notification.Message),
		DelaySeconds: 10,
	}

	resp, err := SendMsg(context.TODO(), client, sMInput)
	if err != nil {
		log.Error("Got an error sending the message: ", err)
		return err
	}
	log.Debug("Message Sent with Id: ", *resp.MessageId)

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

func GetQueueURL(c context.Context, api SQSSendMessageAPI, input *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return api.GetQueueUrl(c, input)
}

func SendMsg(c context.Context, api SQSSendMessageAPI, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return api.SendMessage(c, input)
}
