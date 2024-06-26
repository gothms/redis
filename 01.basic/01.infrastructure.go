package _1_basic

/*
基本架构：一个键值数据库包含什么？

系统观
	想要深入理解和优化 Redis，就必须要对它的总体架构和关键模块有一个全局的认知，然后再深入到具体的技术点
SimpleKV
	思路
		只需要关注整体架构和核心模块
	数据模型
		存什么样的数据
		为什么在有些场景下，原先使用关系型数据库保存的数据，也可以用键值数据库保存
	操作接口
		对数据可以做什么样的操作
		为什么在有些场景中，使用键值数据库又不合适了

可以存哪些数据？
	不同键值数据库支持的 key 类型一般差异不大，而 value 类型则有较大差别
	我们在对键值数据库进行选型时，一个重要的考虑因素是它支持的 value 类型
	例如，Memcached 支持的 value 类型仅为 String 类型，而 Redis 支持的 value 类型包括了 String、哈希表、列表、集合等
	Redis 能够在实际业务场景中得到广泛的应用，就是得益于支持多样化类型的 value
可以对数据做什么操作？
	键值数据库的基本操作集合
		PUT：新写入或更新一个 key-value 对
			有些键值数据库的新写 / 更新操作叫 SET
			新写入和更新虽然是用一个操作接口，但在实际执行时，会根据 key 是否存在而执行相应的新写或更新流程
		GET：根据一个 key 读取相应的 value 值
		DELETE：根据一个 key 删除整个 key-value 对
		SCAN：根据一段 key 的范围返回相应的 value 值
			如查询一个用户在一段时间内的访问记录
	其他操作
		实际业务场景通常还有更加丰富的需求
		例如，在黑白名单应用中，需要判断某个用户是否存在
			如果将该用户的 ID 作为 key，那么，可以增加 EXISTS 操作接口，用于判断某个 key 是否存在
	根据 value 类型操作接口
		例如，Redis 的 value 有列表类型，因此它的接口就要包括对列表 value 的操作
键值对保存在内存还是外存？
	vs
		内存
			保存在内存的好处是读写很快，毕竟内存的访问速度一般都在百 ns 级别
			但是，潜在的风险是一旦掉电，所有的数据都会丢失
		外存
			保存在外存，虽然可以避免数据丢失
			但是受限于磁盘的慢速读写（通常在几 ms 级别），键值数据库的整体性能会被拉低
	键值数据库主要应用场景
		比如，缓存场景下的数据需要能快速访问但允许丢失，那么，用于此场景的键值数据库通常采用内存保存键值数据
		Memcached 和 Redis 都是属于内存键值数据库

SimpleKV 基本组件
	一个键值数据库基本模块
		访问框架、索引模块、操作模块、存储模块
	SimpleKV 基本内部架构
		01.infrastructure_simplekv.jpg
1.访问框架：采用什么访问模式？
	概述
		不同的键值数据库服务器和客户端交互的协议并不相同
		我们在对键值数据库进行二次开发、新增功能时，必须要了解和掌握键值数据库的通信协议，这样才能开发出兼容的客户端
	访问模式通常有两种：
		通过函数库调用的方式供外部应用使用
			如图中的 libsimplekv.so，就是以动态链接库的形式链接到我们自己的程序中，提供键值存储功能
			RocksDB 以动态链接库的形式使用
		通过网络框架以 Socket 通信的形式对外提供键值对操作
			这种形式可以提供广泛的键值存储服务
			如图中，网络框架中包括 Socket Server 和协议解析
			Memcached 和 Redis 则是通过网络框架访问
	通过网络框架提供键值存储服务
		一方面扩大了键值数据库的受用面
		但另一方面，也给键值数据库的性能、运行模型提供了不同的设计选择，带来了一些潜在的问题
	潜在问题示例：
		当客户端发送一个 PUT hello world 的命令后，该命令会被封装在网络包中发送给键值数据库
		I/O 模型设计
			键值数据库网络框架接收到网络包，并按照相应的协议进行解析之后，就可以知道，客户端想写入一个键值对，并开始实际的写入流程
			此时，我们会遇到一个系统设计上的问题，即 I/O 模型设计。不同的 I/O 模型对键值数据库的性能和可扩展性会有不同的影响
			简单来说，就是网络连接的处理、网络请求的解析，以及数据存取的处理，是用一个线程、多个线程，还是多个进程来交互处理呢？该如何进行设计和取舍呢？
		分析
			如果一个线程既要处理网络连接、解析请求，又要完成数据存取，一旦某一步操作发生阻塞，整个线程就会阻塞住，这就降低了系统响应速度
			如果我们采用不同线程处理不同操作，那么，某个线程被阻塞时，其他线程还能正常运行
			但是，不同线程间如果需要访问共享资源，那又会产生线程竞争，也会影响系统效率，这又该怎么办呢？
			所以，这的确是个“两难”选择，需要我们进行精心的设计
2.索引模块：如何定位键值对的位置？
	索引的作用
		让键值数据库根据 key 找到相应 value 的存储位置，进而执行操作
	常见索引类型
		哈希表、B+ 树、字典树等
	概述
		不同键值数据库采用的索引并不相同
		例如，Memcached 和 Redis 采用哈希表作为 key-value 索引
		而 RocksDB 则采用跳表作为内存中 key-value 的索引
		Redis：
			它的 value 支持多种类型，当我们通过索引找到一个key 所对应的 value 后
			仍然需要从 value 的复杂结构（例如集合和列表）中进一步找到我们实际需要的数据，这个操作的效率本身就依赖于它们的实现结构
	哈希表索引
		很大一部分原因在于，其键值数据基本都是保存在内存中的
		而内存的高性能随机访问特性可以很好地与哈希表 O(1) 的操作复杂度相匹配
3.操作模块：不同操作的具体逻辑是怎样的？
	对于不同的操作来说，找到存储位置之后，需要进一步执行的操作的具体逻辑会有所差异
	SimpleKV 的操作模块就实现了不同操作的具体逻辑：
		对于 GET/SCAN 操作而言，此时根据 value 的存储位置返回 value 值即可
		对于 PUT 一个新的键值对数据而言，SimpleKV 需要为该键值对分配内存空间
		对于 DELETE 操作，SimpleKV 需要删除键值对，并释放相应的内存空间，这个过程由分配器完成
	对于 PUT 和 DELETE 两种操作来说，除了新写入和删除键值对，还需要分配和释放内存
4.存储模块：如何实现重启后快速提供服务？
	内存空间管理
		SimpleKV 采用了常用的内存分配器 glibc 的 malloc 和 free，因此，SimpleKV 并不需要特别考虑内存空间的管理问题
		但是，键值数据库的键值对通常大小不一，glibc 的分配器在处理随机的大小内存块分配时，表现并不好
			一旦保存的键值对数据规模过大，就可能会造成较严重的内存碎片问题
		因此，分配器是键值数据库中的一个关键因素
	分配器
		Redis 的内存分配器提供了多种选择，分配效率也不一样
	持久化
		SimpleKV 虽然依赖于内存保存数据，提供快速访问
		但是，我们也希望 SimpleKV 重启后能快速重新提供服务，所以，在 SimpleKV 的存储模块中增加了持久化功能
		不过，鉴于磁盘管理要比内存管理复杂，SimpleKV 就直接采用了文件形式，将键值数据通过调用本地文件系统的操作接口保存在磁盘上
		此时，SimpleKV 只需要考虑何时将内存中的键值数据保存到文件中，就可以了
		ps：Redis为持久化提供了诸多的执行机制和优化改进
	持久化方式
		一种方式是，对于每一个键值对，SimpleKV 都对其进行落盘保存
			这虽然让 SimpleKV 的数据更加可靠，但是，因为每次都要写盘，SimpleKV 的性能会受到很大影响
		另一种方式是，SimpleKV 只是周期性地把内存中的键值数据保存到文件中
			这样可以避免频繁写盘操作的性能影响
			但是，一个潜在的代价是 SimpleKV 的数据仍然有丢失的风险

SimpleKV vs Redis：图示 01.infrastructure_vs.jpg
	几个重要变化：
	Redis 主要通过网络框架进行访问，而不再是动态库了，这也使得 Redis 可以作为一个基础性的网络服务进行访问，扩大了 Redis 的应用范围
	Redis 数据模型中的 value 类型很丰富，因此也带来了更多的操作接口
		例如面向列表的 LPUSH/LPOP，面向集合的 SADD/SREM 等
	Redis 的持久化模块能支持两种方式：日志（AOF）和快照（RDB），这两种持久化方式具有不同的优劣势，影响到 Redis 的访问性能和可靠性
	SimpleKV 是个简单的单机键值数据库
		但是，Redis 支持高可靠集群和高可扩展集群，因此，Redis 中包含了相应的集群功能支撑模块

思考
	和你了解的 Redis 相比，你觉得，SimpleKV 里面还缺少什么功能组件或模块吗？
*/
