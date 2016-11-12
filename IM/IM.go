package IM

import (
	"errors"
	"time"
	"fmt"
	"strings"
)

/*
* IM define a instant message manager
* before start the instance you created, you should first init it or it causes panic
* ---call SetConsumerCallbacks to init the following callbacks which called when message is delayed or convert to persistent
* :::onConsumerExpiredCallback、onRequestedConsumerMissCallback、onMessageTargetMissCallback
* ---call SetChannels to init your channels which handles different messages
*/
type IM struct {
	messageCount uint64
	classifierNum uint32
	channelGroupNum uint32
	channelNum uint32

	channelGroups map[string]*ChannelGroup
	consumerPools map[string]ConsumerPool
	classifiers []MessageClassifier

	consumerInit bool
	expireTime time.Duration

	host string
	port string

	onConsumerExpiredCallback func(*Consumer) error
	onRequestedConsumerMissCallback func(string)(*Consumer, error)
	onMessageTargetMissCallback func(Message, ConsumerPool) error

	communication *Communication
	communicationPath string

	senderPath string

	UserManager *UserManager
	registerURL string
	updateSecretURL string
}

/*
* NewIM create a new IM instance
*/
func NewIM(host string) *IM {
	if !strings.HasPrefix(host, "http://") {
		host = "http://" + host
	}

	return &IM{
		channelGroups		: make(map[string] *ChannelGroup),
		expireTime		: time.Second * 10,

		consumerInit 		: false,

		host			: host,
	}
}

/*
* Send a message to classifier
*/
func (im *IM) SendMessage(m Message) {
	im.messageCount++

	m.SetId(im.messageCount)

	index :=im.messageCount % uint64(im.classifierNum)
	im.classifiers[index].Classify(m)
}

/*
* withdraw message that haven't been read
*/
func (im *IM) ReceiveMessages(id string, t string, race bool) (*MessageReceiver, error) {
	if p, ok := im.consumerPools[t]; ok {
		return p.ReceiveMessages(id, race)
	} else {
		return nil, errors.New("No such message type registered")
	}
}

/*
* set a group of channels to handle a kind of messages
*/
func (im *IM) SetChannel(n ChannelNewer,mt string, channelNum uint32, channelBuffSize uint32, gm GroupManager) {
	if (channelBuffSize < 1) {
		fmt.Println("Channel buffer size must be more than 1")
		fmt.Printf("Channel buffer size is set to default: %d\n", DefaultChannelBufferSize)
		channelBuffSize = DefaultChannelBufferSize
	}

	im.channelGroupNum ++
	im.channelNum += channelNum
	im.channelGroups[mt] = NewChannelGroup(n, mt, channelNum, channelBuffSize)
	im.channelGroups[mt].gm = gm
}
func (im *IM) SetClassifierNum(cn uint32) {
	if cn < 1 {
		fmt.Println("At least one message classifier is needed")
		fmt.Printf("Classifier num is set to default: %d\n", defaultClassifierNum)
		cn = defaultClassifierNum
	}
	im.classifierNum = cn
}

/*
* Settings about communication stuff
*/
func (im *IM) SetCommunicationPath(path string) {
	im.communication = NewCommunication(im)
	im.communicationPath = path
}
func (im *IM) FetchFile(name string) ([]byte, error) {
	return im.communication.FetchFile(name)
}
func (im *IM) SetSenderPath(path string) {
	im.senderPath = path
}

/*
* Settings about user manager
*/
func (im *IM) SetUserManager(secretKey string, registerURL string, updateSecretURL string) {
	im.UserManager = NewUserManager(secretKey)
	im.registerURL = registerURL
	im.updateSecretURL = updateSecretURL
}
func (im *IM) Validate(validation string) (string, error) {
	return im.UserManager.Validate(validation)
}

/*
* consumer callbacks are called in consumer pool when a message can't find its target consumer or
* the message is going to be expired or a consumer stored in database need to be loaded to cache pool
*/
func (im *IM) SetConsumerCallbacks(onConsumerExpiredCallback func(*Consumer) error,
	onRequestedConsumerMissCallback func(string)(*Consumer, error),
	onMessageTargetMissCallback func(Message, ConsumerPool) error) {

	e := errors.New("Message not handled")

	if (onRequestedConsumerMissCallback == nil) {
		onRequestedConsumerMissCallback = func(id string) (*Consumer, error) { return NewConsumer(id, defaultBuffSize), e }
	}
	if (onMessageTargetMissCallback == nil) {
		onMessageTargetMissCallback = func(m Message, p ConsumerPool) error { return e }
	}
	if (onConsumerExpiredCallback == nil) {
		onConsumerExpiredCallback = func(c *Consumer) error { return e }
	}

	im.onConsumerExpiredCallback = onConsumerExpiredCallback
	im.onMessageTargetMissCallback = onMessageTargetMissCallback
	im.onRequestedConsumerMissCallback = onRequestedConsumerMissCallback

	im.consumerInit = true
}

/*
* init IM struct
*/
func (im *IM) init() {
	im.classifiers = make([]MessageClassifier, im.classifierNum)
	for i := range im.classifiers {
		im.classifiers[i] = NewMessageClassifier()
	}

	im.consumerPools = make(map[string]ConsumerPool)
	for _, g := range im.channelGroups {
		g.SetMessageClassifiers(im.classifiers)
		im.consumerPools[g.mt] = NewConsumerPool(im.onConsumerExpiredCallback, im.onRequestedConsumerMissCallback, im.onMessageTargetMissCallback)
		im.consumerPools[g.mt].Start(g.sendingMessage)
		g.StartChannels()
		if g.gm != nil {
			g.gm.StartManage(g)
		}
	}

	im.UserManager.StartExpireCheck(time.Minute * 10)

	route(im)
}

/*
* start the im manager
*/
func (im *IM) Start() {
	if len(im.channelGroups) == 0 {
		panic("No channels set")
	}
	if im.consumerInit == false {
		panic("Consumer callbacks not set")
	}
	if im.communication == nil {
		panic("Communication is not set")
	}
	if im.senderPath == "" {
		panic("Sender path not set")
	}
	if im.UserManager == nil {
		panic("User maanger is not set")
	}

	im.init()
}
