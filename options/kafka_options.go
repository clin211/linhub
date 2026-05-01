package options

import (
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/segmentio/kafka-go/snappy"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	stringsutil "github.com/clin211/linhub/util/strings"
)

var _ IOptions = (*KafkaOptions)(nil)

type logger struct {
	v int32
}

func (l *logger) Printf(format string, args ...any) {
	klog.V(klog.Level(l.v)).Infof(format, args...)
}

type WriterOptions struct {
	// 投递一条消息所允许的最大尝试次数。
	//
	// 默认最多尝试 10 次。
	MaxAttempts int `mapstructure:"max-attempts"`

	// 在收到生产请求响应前，分区副本需要的确认数量。默认值为 -1，
	// 表示等待所有副本确认；大于 0 的值表示需要多少个副本确认消息才视为成功。
	//
	// 此版本的 kafka-go（v0.3）不支持 0 required acks，
	// 因为在 Kafka 协议中实现该功能存在一些内部复杂性。
	// 如果你确实需要该功能，则需要升级到 v0.4。
	RequiredAcks int `mapstructure:"required-acks"`

	// 将该标志设置为 true 会使 WriteMessages 方法永不阻塞。
	// 这也意味着错误将被忽略，因为调用方不会收到返回值。
	// 仅在你不关心消息是否被写入 Kafka 的保证时使用此选项。
	Async bool `mapstructure:"async"`

	// 在发送到分区前可缓冲的消息数量上限。
	//
	// 默认目标批次大小为 100 条消息。
	BatchSize int `mapstructure:"batch-size"`

	// 不完整消息批次刷新到 Kafka 的频率时间限制。
	//
	// 默认至少每秒刷新一次。
	BatchTimeout time.Duration `mapstructure:"batch-timeout"`

	// 在发送到分区前请求的最大字节数上限。
	//
	// 默认使用 Kafka 默认值 1048576。
	BatchBytes int `mapstructure:"batch-bytes"`
}

type ReaderOptions struct {
	// GroupID 是可选的消费者组 ID。如果指定了 GroupID，
	// 则不应同时指定 Partition（例如：0）
	GroupID string `mapstructure:"group-id"`

	// GroupTopics 允许指定多个 topic，但只能与 GroupID 一起使用，
	// 因为它是消费者组的特性。因此，如果设置了 GroupID，
	// 则必须定义 Topic 或 GroupTopics 中的一个。
	// GroupTopics []string

	// 要从中读取消息的分区。Partition 与 GroupID 二者只能选其一，不能同时指定。
	Partition int `mapstructure:"partition"`

	// 内部消息队列的容量，未设置时默认为 100。
	QueueCapacity int `mapstructure:"queue-capacity"`

	// MinBytes 告知 broker 消费者将接受的最小批次大小。
	// 在低流量 topic 上消费时，将其设置过高可能会导致 broker
	// 没有足够数据满足定义的最小值时出现延迟投递。
	//
	// 默认值：1
	MinBytes int `mapstructure:"min-bytes"`

	// MaxBytes 告知 broker 消费者将接受的最大批次大小。
	// broker 将截断消息以满足该最大值，因此请选择一个
	// 足以容纳你最大消息大小的值。
	//
	// 默认值：1MB
	MaxBytes int `mapstructure:"max-bytes"`

	// 从 Kafka 拉取消息批次时，等待新数据到达的最大时长。
	//
	// 默认值：10s
	MaxWait time.Duration `mapstructure:"max-wait"`

	// ReadBatchTimeout 是从 Kafka 消息批次中拉取消息时的等待时长。
	//
	// 默认值：10s
	ReadBatchTimeout time.Duration `mapstructure:"read-batch-timeout"`

	// ReadLagInterval 设置 reader lag 的更新频率。
	// 将该字段设置为负值会禁用 lag 上报。
	// ReadLagInterval time.Duration

	// HeartbeatInterval 设置可选的 reader 发送消费者组心跳更新的频率。
	//
	// 默认值：3s
	//
	// 仅在设置了 GroupID 时使用
	HeartbeatInterval time.Duration `mapstructure:"heartbeat-interval"`

	// CommitInterval 表示向 broker 提交 offset 的间隔。
	// 如果为 0，则同步处理提交。
	//
	// 默认值：0
	//
	// 仅在设置了 GroupID 时使用
	CommitInterval time.Duration `mapstructure:"commit-interval"`

	// RebalanceTimeout 可选地设置协调者等待成员加入再平衡的时长。
	// 对于负载较高的 Kafka 服务器，将该值设置得更高会更有用。
	//
	// 默认值：30s
	//
	// 仅在设置了 GroupID 时使用
	RebalanceTimeout time.Duration `mapstructure:"rebalance-timeout"`

	// StartOffset 决定消费者组在发现没有已提交 offset 的分区时从何处开始消费。
	// 如果为非零值，必须设置为 FirstOffset 或 LastOffset 之一。
	//
	// 默认值：FirstOffset
	//
	// 仅在设置了 GroupID 时使用
	StartOffset int64 `mapstructure:"start-offset"`

	// 在投递错误之前所允许的最大尝试次数。
	//
	// 默认尝试 3 次。
	MaxAttempts int `mapstructure:"max-attempts"`
}

// KafkaOptions 定义 Kafka 集群的选项。
// 包含 kafka-go reader 和 writer 的通用选项。
type KafkaOptions struct {
	// kafka-go reader 和 writer 通用选项
	Brokers       []string      `mapstructure:"brokers"`
	Topic         string        `mapstructure:"topic"`
	ClientID      string        `mapstructure:"client-id"`
	Timeout       time.Duration `mapstructure:"timeout"`
	TLSOptions    *TLSOptions   `mapstructure:"tls"`
	SASLMechanism string        `mapstructure:"mechanism"`
	Username      string        `mapstructure:"username"`
	Password      string        `mapstructure:"password"`
	Algorithm     string        `mapstructure:"algorithm"`
	Compressed    bool          `mapstructure:"compressed"`

	// kafka-go writer 选项
	WriterOptions WriterOptions `mapstructure:"writer"`

	// kafka-go reader 选项
	ReaderOptions ReaderOptions `mapstructure:"reader"`
}

// NewKafkaOptions 创建一个 `zero` 值实例。
func NewKafkaOptions() *KafkaOptions {
	return &KafkaOptions{
		TLSOptions: NewTLSOptions(),
		Timeout:    3 * time.Second,
		WriterOptions: WriterOptions{
			RequiredAcks: 1,
			MaxAttempts:  10,
			Async:        true,
			BatchSize:    100,
			BatchTimeout: 1 * time.Second,
			BatchBytes:   1 * MiB,
		},
		ReaderOptions: ReaderOptions{
			QueueCapacity:     100,
			MinBytes:          1,
			MaxBytes:          1 * MiB,
			MaxWait:           10 * time.Second,
			ReadBatchTimeout:  10 * time.Second,
			HeartbeatInterval: 3 * time.Second,
			CommitInterval:    0 * time.Second,
			RebalanceTimeout:  30 * time.Second,
			StartOffset:       kafka.FirstOffset,
			MaxAttempts:       3,
		},
	}
}

// Validate 校验传递给 KafkaOptions 的命令行标志。
func (o *KafkaOptions) Validate() []error {
	errs := []error{}

	if len(o.Brokers) == 0 {
		errs = append(errs, fmt.Errorf("kafka broker can not be empty"))
	}

	if !o.TLSOptions.UseTLS && o.SASLMechanism != "" {
		errs = append(errs, fmt.Errorf("SASL-Mechanism is setted but use_ssl is false"))
	}

	if !stringsutil.StringIn(strings.ToLower(o.SASLMechanism), []string{"plain", "scram", ""}) {
		errs = append(errs, fmt.Errorf("doesn't support '%s' SASL mechanism", o.SASLMechanism))
	}

	if o.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("--kafka.timeout cannot be negative"))
	}

	if o.ReaderOptions.GroupID != "" && o.ReaderOptions.Partition != 0 {
		errs = append(errs, fmt.Errorf("either Partition or GroupID may be assigned, but not both"))
	}

	if o.WriterOptions.BatchTimeout <= 0 {
		errs = append(errs, fmt.Errorf("--kafka.writer.batch-timeout cannot be negative"))
	}

	errs = append(errs, o.TLSOptions.Validate()...)

	return errs
}

// AddFlags 将与特定 APIServer 的 Kafka 存储相关的命令行标志添加到指定的 FlagSet。
func (o *KafkaOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	o.TLSOptions.AddFlags(fs, fullPrefix+".tls")

	fs.StringSliceVar(&o.Brokers, fullPrefix+".brokers", o.Brokers, "用于发现 Kafka 集群可用分区的 broker 列表。")
	fs.StringVar(&o.Topic, fullPrefix+".topic", o.Topic, "writer/reader 将生产/消费消息所使用的 topic。")
	fs.StringVar(&o.ClientID, fullPrefix+".client-id", o.ClientID, "由该 Dialer 建立的客户端连接的唯一标识符。")
	fs.DurationVar(&o.Timeout, fullPrefix+".timeout", o.Timeout, "拨号等待连接建立完成的最大时长。")
	fs.StringVar(&o.SASLMechanism, fullPrefix+".mechanism", o.SASLMechanism, "配置 Dialer 使用 SASL 认证。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username, "Kafka 集群的用户名。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password, "Kafka 集群的密码。")
	fs.StringVar(&o.Algorithm, fullPrefix+".algorithm", o.Algorithm, "用于创建 sasl.Mechanism 的算法。")
	fs.BoolVar(&o.Compressed, fullPrefix+".compressed", o.Compressed, "compressed 用于指定是否压缩 Kafka 消息。")
	fs.IntVar(&o.WriterOptions.RequiredAcks, fullPrefix+".required-acks", o.WriterOptions.RequiredAcks, ""+
		"在收到生产请求响应前，分区副本需要的确认数量。")
	fs.IntVar(&o.WriterOptions.MaxAttempts, fullPrefix+".writer.max-attempts", o.WriterOptions.MaxAttempts, ""+
		"投递一条消息所允许的最大尝试次数。")
	fs.BoolVar(&o.WriterOptions.Async, fullPrefix+".writer.async", o.WriterOptions.Async, "投递一条消息所允许的最大尝试次数。")
	fs.IntVar(&o.WriterOptions.BatchSize, fullPrefix+".writer.batch-size", o.WriterOptions.BatchSize, ""+
		"在发送到分区前可缓冲的消息数量上限。")
	fs.DurationVar(&o.WriterOptions.BatchTimeout, fullPrefix+".writer.batch-timeout", o.WriterOptions.BatchTimeout, ""+
		"不完整消息批次刷新到 Kafka 的频率时间限制。")
	fs.IntVar(&o.WriterOptions.BatchBytes, fullPrefix+".writer.batch-bytes", o.WriterOptions.BatchBytes, ""+
		"在发送到分区前请求的最大字节数上限。")
	fs.StringVar(&o.ReaderOptions.GroupID, fullPrefix+".reader.group-id", o.ReaderOptions.GroupID, ""+
		"GroupID 是可选的消费者组 ID。如果指定了 GroupID，则不应同时指定 Partition（例如：0）。")
	fs.IntVar(&o.ReaderOptions.Partition, fullPrefix+".reader.partition", o.ReaderOptions.Partition, "要从中读取消息的分区。")
	fs.IntVar(&o.ReaderOptions.QueueCapacity, fullPrefix+".reader.queue-capacity", o.ReaderOptions.QueueCapacity, ""+
		"内部消息队列的容量，未设置时默认为 100。")
	fs.IntVar(&o.ReaderOptions.MinBytes, fullPrefix+".reader.min-bytes", o.ReaderOptions.MinBytes, ""+
		"MinBytes 告知 broker 消费者将接受的最小批次大小。")
	fs.IntVar(&o.ReaderOptions.MaxBytes, fullPrefix+".reader.max-bytes", o.ReaderOptions.MaxBytes, ""+
		"MaxBytes 告知 broker 消费者将接受的最大批次大小。")
	fs.DurationVar(&o.ReaderOptions.MaxWait, fullPrefix+".reader.max-wait", o.ReaderOptions.MaxWait, ""+
		"从 Kafka 拉取消息批次时，等待新数据到达的最大时长。")
	fs.DurationVar(&o.ReaderOptions.ReadBatchTimeout, fullPrefix+".reader.read-batch-timeout", o.ReaderOptions.ReadBatchTimeout, ""+
		"ReadBatchTimeout 是从 Kafka 消息批次中拉取消息时的等待时长。")
	fs.DurationVar(&o.ReaderOptions.HeartbeatInterval, fullPrefix+".reader.heartbeat-interval", o.ReaderOptions.HeartbeatInterval, ""+
		"HeartbeatInterval 设置可选的 reader 发送消费者组心跳更新的频率。")
	fs.DurationVar(&o.ReaderOptions.CommitInterval, fullPrefix+".reader.commit-interval", o.ReaderOptions.CommitInterval, ""+
		"CommitInterval 表示向 broker 提交 offset 的间隔。")
	fs.DurationVar(&o.ReaderOptions.RebalanceTimeout, fullPrefix+".reader.rebalance-timeout", o.ReaderOptions.RebalanceTimeout, ""+
		"RebalanceTimeout 可选地设置协调者等待成员加入再平衡的时长。")
	fs.Int64Var(&o.ReaderOptions.StartOffset, fullPrefix+".reader.start-offset", o.ReaderOptions.StartOffset, ""+
		"StartOffset 决定消费者组在发现没有已提交 offset 的分区时从何处开始消费。")
	fs.IntVar(&o.ReaderOptions.MaxAttempts, fullPrefix+".reader.max-attempts", o.ReaderOptions.MaxAttempts, ""+
		"在投递错误之前所允许的最大尝试次数。")
}

func (o *KafkaOptions) GetMechanism() (sasl.Mechanism, error) {
	var mechanism sasl.Mechanism

	switch o.SASLMechanism {
	case "":
		break
	case "PLAIN", "plain":
		mechanism = plain.Mechanism{Username: o.Username, Password: o.Password}
	case "SCRAM", "scram":
		algorithm := scram.SHA256
		if o.Algorithm == "sha-512" || o.Algorithm == "SHA-512" {
			algorithm = scram.SHA512
		}
		var err error
		mechanism, err = scram.Mechanism(algorithm, o.Username, o.Password)
		if err != nil {
			return nil, fmt.Errorf("failed initialize kafka mechanism: %w", err)
		}
	default:
	}

	return mechanism, nil
}

func (o *KafkaOptions) Dialer() (*kafka.Dialer, error) {
	tlsConfig, err := o.TLSOptions.TLSConfig()
	if err != nil {
		return nil, err
	}

	mechanism, err := o.GetMechanism()
	if err != nil {
		return nil, err
	}

	return &kafka.Dialer{
		Timeout:       o.Timeout,
		ClientID:      o.ClientID,
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}, nil
}

func (o *KafkaOptions) Writer() (*kafka.Writer, error) {
	dialer, err := o.Dialer()
	if err != nil {
		return nil, err
	}

	// Kafka writer 连接配置
	config := kafka.WriterConfig{
		Brokers:      o.Brokers,
		Topic:        o.Topic,
		Balancer:     &kafka.LeastBytes{},
		Dialer:       dialer,
		WriteTimeout: o.Timeout,
		ReadTimeout:  o.Timeout,

		Async:        o.WriterOptions.Async,
		BatchSize:    o.WriterOptions.BatchSize,
		BatchBytes:   o.WriterOptions.BatchBytes,
		BatchTimeout: o.WriterOptions.BatchTimeout,
		MaxAttempts:  o.WriterOptions.MaxAttempts,
		Logger:       &logger{4},
		ErrorLogger:  &logger{1},
	}

	if o.Compressed {
		config.CompressionCodec = snappy.NewCompressionCodec()
	}

	kafkaWriter := kafka.NewWriter(config)
	return kafkaWriter, nil
}
