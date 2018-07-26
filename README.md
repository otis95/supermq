# supermq
High-Performance MQTT Server and  MQTT cluster

**This project was developed based on surgermq.(https://github.com/surgemq/surgemq)**

**surgemq is not support cluster and consumer group.**

### MQTT

According to the [MQTT spec](http://docs.oasis-open.org/mqtt/mqtt/v3.1.1/mqtt-v3.1.1.html):

> MQTT is a Client Server publish/subscribe messaging transport protocol. It is light weight, open, simple, and designed so as to be easy to implement. These characteristics make it ideal for use in many situations, including constrained environments such as for communication in Machine to Machine (M2M) and Internet of Things (IoT) contexts where a small code footprint is required and/or network bandwidth is at a premium.
>
> The protocol runs over TCP/IP, or over other network protocols that provide ordered, lossless, bi-directional connections. Its features include:
> 
> * Use of the publish/subscribe message pattern which provides one-to-many message distribution and decoupling of applications.
> * A messaging transport that is agnostic to the content of the payload.
> * Three qualities of service for message delivery:
>   * "At most once", where messages are delivered according to the best efforts of the operating environment. Message loss can occur. This level could be used, for example, with ambient sensor data where it does not matter if an individual reading is lost as the next one will be published soon after.
>   * "At least once", where messages are assured to arrive but duplicates can occur.
>   * "Exactly once", where message are assured to arrive exactly once. This level could be used, for example, with billing systems where duplicate or lost messages could lead to incorrect charges being applied.
> * A small transport overhead and protocol exchanges minimized to reduce network traffic.
> * A mechanism to notify interested parties when an abnormal disconnection occurs.

There's some very large implementation of MQTT such as [Facebook Messenger](https://www.facebook.com/notes/facebook-engineering/building-facebook-messenger/10150259350998920). There's also an active Eclipse project, [Paho](https://eclipse.org/paho/), that provides scalable open-source client implementations for many different languages, including C/C++, Java, Python, JavaScript, C# .Net and Go.

### Features, Limitations, and Future

Details you can view [surgemq](https://github.com/surgemq/surgemq).

### Performance

It will be done later.

### Cluster

Because I don't have experience to develop message queue cluster, so I refer to [NATS](https://github.com/nats-io/gnatsd) to do the mqtt cluster.

**Architecture diagram**

![Image](https://raw.githubusercontent.com/578157900/supermq/master/images/router.png)

### Queue

Consumer group.MQTT clients subscribe a regular topic,just one client will receive message in random.

**Architecture diagram**

![Image](https://raw.githubusercontent.com/578157900/supermq/master/images/queue.jpg)

**Focus**

* Clusters use qos 0 when they publish and subscribe topic.I think too much shake hands will reduce the performance of server.
* Clusters's connections is not supported TLS and SSL now.
* '*' and '+' are allowed in queue name. Queue name's length < 10.Example: $g/:name/xxxxx/xxxx  name is '**123++' is ok.
* Queue is not been supported in mqtt cluster.

### Examples

**Start**

```
  git clone https://github.com/578157900/supermq

  go  build 

  supermq -c appserver.conf
```

**Result**

subscribe in 1883 server
![Image](https://raw.githubusercontent.com/578157900/supermq/master/images/1883.png)

publish in 1884 server
![Image](https://raw.githubusercontent.com/578157900/supermq/master/images/1884.png)

### Conclusion
* I made this project in short time so i believe this project will have too many bugs.And I hope everyone can help me improve the stability and usability of this service, Thank you!!! Please call me!!!

### Contribute

[panyingyun](https://github.com/panyingyun)


