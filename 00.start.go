package redis

/*
这样学Redis，才能技高一筹

目标
	设计一个单机千万级吞吐量的键值数据库
HiKV
	https://www.usenix.org/conference/atc17/technical-sessions/presentation/xia

性能相关问题
	问题
		为了保证数据的可靠性，Redis 需要在磁盘上读写 AOF 和 RDB，但在高并发场景里，这就会直接带来两个新问题：
		一个是写 AOF 和 RDB 会造成 Redis 性能抖动
		另一个是 Redis 集群数据同步和实例恢复时，读 RDB 比较慢，限制了同步和恢复速度
	一个方案
		使用非易失内存 NVM，因为它既能保证高速的读写，又能快速持久化数据
四个方面的“坑”
	CPU 使用上的“坑”，例如数据结构的复杂度、跨 CPU 核的访问
	内存使用上的“坑”，例如主从同步和 AOF 的内存竞争
	存储持久化上的“坑”，例如在 SSD 上做快照的性能抖动
	网络通信上的“坑”，例如多实例时的异常网络丢

为什么懂得了一个个技术点，却依然用不好 Redis？
	系统观
		只关注零散的技术点，没有建立起一套完整的知识框架，缺乏系统观，但是，系统观其实是至关重要的
		从某种程度上说，在解决问题时，拥有了系统观，就意味着你能有依据、有章法地定位和解决问题
	案例：在评估性能时，仅仅看平均延迟已经不足够
		假设 Redis 处理了 100 个请求，99 个请求的响应时间都是 1s，而有一个请求的响应时间是 100s
		那么，如果看平均延迟，这 100 个请求的平均延迟是 1.99s，但是对于这个响应时间是 100s 的请求而言，它对应的用户体验将是非常糟糕的
		如果有 100 万个请求，哪怕只有 1% 的请求是 100s，这也对应了 1 万个糟糕的用户体验
		这 1% 的请求延迟就属于长尾延迟
	把 Redis 的长尾延迟维持在一定阈值以下，问题分析
		线程模型：对于单线程的 Redis 而言，任何阻塞性操作都会导致长尾延迟的产生
		Redis 网络框架：Redis 网络 IO 使用了 IO 复用机制，并不会阻塞在单个客户端上
		还有，键值对数据结构、持久化机制下的 fork 调用、主从库同步时的AOF 重写，以及缓冲区溢出等多个方面
	两大维度，三大主线：Redis知识全景图.jpg
		系统维度
		应用维度
		高性能
		高可靠
		高可扩展

系统维度
	需要了解 Redis 的各项关键技术的设计原理，这些能够为你判断和推理问题打下坚实的基础
	而且，你还能从中掌握一些优雅的系统设计规范
	例如 run-to-complete 模型、epoll 网络模型，这些可以应用到你后续的系统开发实践中
应用维度
	两种方式学习: “应用场景驱动”和“典型案例驱动”，一个是“面”的梳理，一个是“点”的掌握
	两大应用场景：缓存、集群
		缓存：缓存机制、缓存替换、缓存异常等问题
		集群
	典型案例驱动
		原因
			Redis 丰富的数据模型，就导致它有很多零碎的应用场景，很多很杂
			有一些问题隐藏得比较深，只有特定的业务场景下（比如亿级访问压力场景）才会出现，并不是普遍现象
		“三高”案例
			多家大厂在万亿级访问量和万亿级数据量的情况下对 Redis 的深度优化，解读这些优化实践，需要你透彻的理解 Redis
			还可以梳理一些方法论，做成 Checklist，就像是一个个锦囊，之后当你遇到问题的时候，就可以随时拿出自己的“锦囊妙计”解决问题
主线技术
	高性能主线：包括线程模型、数据结构、持久化、网络框架
	高可靠主线：包括主从复制、哨兵机制
	高可扩展主线：包括数据分片、负载均衡
问题&技术
	Redis问题画像图.jpg
	在学习和使用的过程中，完全可以根据自己的方式，完善这张画像图，把自己实践或掌握到的新知识点
	按照“问题 --> 主线 --> 技术点”的方式梳理出来，放到这张图上

基础篇：打破技术点之间的壁垒，建立网状知识结构
	数据结构、线程模型、持久化等几根“顶梁柱”
实践篇：场景和案例驱动，取人之长，梳理出一套属于自己的“武林秘籍”
	案例驱动
		数据结构的合理使用、避免请求阻塞和抖动、避免内存竞争和提升内存使用效率的关键技巧
	场景驱动
		缓存和集群两大场景
		缓存：缓存基本原理及淘汰策略，还有雪崩、穿透、污染等异常情况
		集群：集群方案优化、数据一致性、高并发访问等问题，可行的解决方案
未来篇：具有前瞻性，解锁新特性
	Redis 6.0：多线程等新特性
selective
	运维工具、定制化客户端开发的方法、经典的学习资料

Redis 是一个非常优秀的系统，它在 CPU 使用、内存组织、存储持久化和网络通信这四大方面的设计非常经典
*/
