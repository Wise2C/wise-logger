# wise-logger 
容器化应用的日志收集工具
***by wise2c***

## 快速启动

* `docker`原生启动

    [docker-compose](https://github.com/docker/compose/releases/tag/1.8.0) `up -d`

* `rancher`平台上启动

    [rancher-compose](https://github.com/rancher/rancher-compose/releases/tag/v0.9.2) `--url http://server_ip:8080 --access-key <username_of_environment_api_key> --secret-key <password_of_environment_api_key> up -d`

## 特性

* `对现有应用无侵入`。现有的写文件保存日志的应用在容器化时无需任何修改，启动容器时加相应设置即可将输出日志纳入管理
* `监听新增日志容器`。监听本机的docker event，有符合条件的容器生成时，在日志收集的配置文件中为新容器添加匹配的规则
* `监听配置模板`。生成日志收集的配置文件的模板如果有更新，会重新生成配置（已将模板放入ETCD中）
* `获取日志容器在业务逻辑上的从属关系`。获取日志容器的`stack`、`service`、`index`并将这些信息注入到日志中(`rancher`)
* `保持日志连贯性`。容器在跨主机调度后，`stack`、`service`、`index`仍保持不变，保证了日志在逻辑上的连贯性(`rancher`)

## 部署约定

* 每个应用容器，有一个专属的日志容器，在rancher中，日志容器要作为应用容器的`sidekicks`
    * 确保不同应用之间不会产生日志路径冲突
    * 确保被收集到的日志有明确的来源
	* 确保应用容器和日志容器始终在同一台宿主机上，且日志容器的标签中有应用的`service`信息
* 每个日志容器都有`logtype`标签
    * 做为被`wise-logger`识别的标识
    * 依据标签内容生成不同的日志采集配置
    * 多个标签内容用分号隔开，如：`logtype=xxx;yyy;zzz`
* 每个日志容器只能有一个`local driver volume`
* 模板文件中要涵盖每一种可能的`logtype`