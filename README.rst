短信中继 smsrelay 
=====================

短信中继smsrelay的目标是隐藏不同短信提供商接口的差异，
提供一个统一的封装使得在切换短信提供商时不需要更改应用层代码和配置，
同时提供一些附加的功能来满足常见的运营需求。

支持的短信网关
--------------

- 梦网科技
- 亿美软通

功能
----

- 余额查询，发送短信，收取用户上行短信，收取短信状态报告(TODO)
- 可配置多个中继通道，通道可以限速，并用于不同的用途（如用户通知和促销使用不同的通道防止相互影响）
- 可配置多个用户，每用户使用不同的中继通道，并对可发送短信的时段进行限制
- 临时连接失败时自动重发

编译安装
---------

需要预先安装Go编译器并设置好GOPATH环境变量，参见 http://golang.org/doc/install 

::

	go install github.com/hukeli/smsrelay


配置
----

TODO


API
----

发送短信
^^^^^^^^

::

	POST /send HTTP/1.1
	Host: sms.example.com
	Date: Wed, 12 Oct 2009 17:50:00 GMT
	Content-Type: application/x-www-form-urlencoded

	user=testsms&password=ofijr90fgjoir01rgmb&mobile=18618618600&message=测试

余额查询
^^^^^^^^

::

	GET /balance HTTP/1.1
	Host: sms.example.com
	Date: Wed, 12 Oct 2009 17:50:00 GMT



