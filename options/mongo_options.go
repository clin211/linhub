package options

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/pflag"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var _ IOptions = (*MongoOptions)(nil)

// MongoOptions 包含连接 MongoDB 服务器的选项。
type MongoOptions struct {
	URL        string        `json:"url" mapstructure:"url"`
	Database   string        `json:"database" mapstructure:"database"`
	Collection string        `json:"collection" mapstructure:"collection"`
	Username   string        `json:"username" mapstructure:"username"`
	Password   string        `json:"password" mapstructure:"password"`
	Timeout    time.Duration `json:"timeout" mapstructure:"timeout"`
	TLSOptions *TLSOptions   `json:"tls" mapstructure:"tls"`
}

// NewMongoOptions 创建一个 `zero` 值实例。
func NewMongoOptions() *MongoOptions {
	return &MongoOptions{
		Timeout:    30 * time.Second,
		TLSOptions: NewTLSOptions(),
	}
}

// Validate 校验传递给 MongoOptions 的命令行标志。
func (o *MongoOptions) Validate() []error {
	errs := []error{}

	if _, err := url.Parse(o.URL); err != nil {
		errs = append(errs, fmt.Errorf("unable to parse connection URL: %w", err))
	}

	if o.Database == "" {
		errs = append(errs, fmt.Errorf("--mongo.database can not be empty"))
	}

	if o.Collection == "" {
		errs = append(errs, fmt.Errorf("--mongo.collection can not be empty"))
	}

	if o.TLSOptions != nil {
		errs = append(errs, o.TLSOptions.Validate()...)
	}

	return errs
}

// AddFlags 将与特定 APIServer 的 MongoDB 存储相关的命令行标志添加到指定的 FlagSet。
func (o *MongoOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	o.TLSOptions.AddFlags(fs, fullPrefix+".tls")

	fs.DurationVar(&o.Timeout, fullPrefix+".timeout", o.Timeout, "拨号等待连接建立完成的最大时长。")
	fs.StringVar(&o.URL, fullPrefix+".url", o.URL, "MongoDB 服务器地址。")
	fs.StringVar(&o.Database, fullPrefix+".database", o.Database, "MongoDB 数据库名称。")
	fs.StringVar(&o.Collection, fullPrefix+".collection", o.Collection, "MongoDB 集合名称。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username, "MongoDB 数据库的用户名（可选）。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password, "MongoDB 数据库的密码（可选）。")
}

// NewClient 基于提供的选项创建一个新的 MongoDB 客户端。
func (o *MongoOptions) NewClient() (*mongo.Client, error) {
	// 设置客户端选项
	opts := options.Client().ApplyURI(o.URL).SetReadPreference(readpref.Primary())
	if o.Timeout > 0 {
		opts.SetConnectTimeout(o.Timeout).SetSocketTimeout(o.Timeout).SetServerSelectionTimeout(o.Timeout)
	}

	if o.Username != "" || o.Password != "" {
		opts.SetAuth(options.Credential{
			AuthSource: o.Database,
			Username:   o.Username,
			Password:   o.Password,
		})
	}

	if o.TLSOptions != nil {
		tlsConf, err := o.TLSOptions.TLSConfig()
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsConf)
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()

	// 连接到 MongoDB
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 通过 Ping MongoDB 服务器以检查连接是否正常
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	return client, nil
}
