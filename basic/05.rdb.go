package redis

/*
内存快照：宕机后，Redis如何实现快速恢复？

有没有既可以保证可靠性，还能在宕机时实现快速恢复的其他方法呢？

内存快照（Redis DataBase）
	简介
		指内存中的数据在某一个时刻的状态记录
			类似于照片，当你给朋友拍照时，一张照片就能把朋友一瞬间的形象完全记下来
		对 Redis 来说，它实现类似照片记录效果的方式，就是把某一时刻的状态以文件的形式写到磁盘上，也就是快照
		这样一来，即使宕机，快照文件也不会丢失，数据的可靠性也就得到了保证
		这个快照文件就称为 RDB 文件，其中，RDB 就是 Redis DataBase 的缩写
	vs
		和 AOF 相比，RDB 记录的是某一时刻的数据，并不是操作，所以，在做数据恢复时，我们可以直接把 RDB 文件读入内存，很快地完成恢复
	两个关键问题：
		对哪些数据做快照？这关系到快照的执行效率问题
		做快照时，数据还能被增删改吗？这关系到 Redis 是否被阻塞，能否同时正常处理请求
	类比
		如何取景？也就是说，我们打算把哪些人、哪些物拍到照片中
		在按快门前，要记着提醒朋友不要乱动，否则拍出来的照片就模糊了

给哪些内存数据做快照？
	全量快照
		Redis 的数据都在内存中，为了提供所有数据的可靠性保证，它执行的是全量快照
		也就是说，把内存中的所有数据都记录到磁盘中
		好处是，一次性记录了所有数据，一个都不少
	问题分析
		给内存的全量数据做快照，把它们全部写入磁盘也会花费很多时间
		而且，全量数据越多，RDB 文件就越大，往磁盘上写数据的时间开销就越大
		针对任何操作，都会提一个灵魂之问：“它会阻塞主线程吗?”
		RDB 文件的生成是否会阻塞主线程，这就关系到是否会降低 Redis 的性能
	save 和 bgsave：Redis 提供了两个命令来生成 RDB 文件，分别是 save 和 bgsave
		save：在主线程中执行，会导致阻塞
		bgsave：创建一个子进程，专门用于写入 RDB 文件，避免了主线程的阻塞，这也是 Redis RDB 文件生成的默认配置
			通过 bgsave 命令来执行全量快照，这既提供了数据的可靠性保证，也避免了对 Redis 的性能影响
快照时数据能修改吗?
	需求分析
		如果快照执行期间数据不能被修改，是会有潜在问题的
		在做快照的时间里，如果数据都不能被修改，Redis 就不能处理对这些数据的写操作，那无疑就会给业务服务造成巨大的影响
		避免阻塞和正常处理写操作并不是一回事
			bgsave 避免了主线程的阻塞
			此时，主线程的确没有阻塞，可以正常接收请求，但是，为了保证快照完整性，它只能处理读操作，因为不能修改正在执行快照的数据
		为了快照而暂停写操作，肯定是不能接受的
	操作系统提供的写时复制技术（Copy-On-Write, COW）
		在执行快照的同时，正常处理写操作
	写时复制机制
		图示 05.rdb_bgsave.jpg
			bgsave 子进程是由主线程 fork 生成的，可以共享主线程的所有内存数据
			bgsave 子进程运行后，开始读取主线程的内存数据，并把它们写入 RDB 文件
		读
			如果主线程对这些数据也都是读操作（例如图中的键值对 A），那么，主线程和 bgsave 子进程相互不影响
		写
			如果主线程要修改一块数据（例如图中的键值对 C），那么，这块数据就会被复制一份，生成该数据的副本
			然后，bgsave 子进程会把这个副本数据写入 RDB 文件，而在这个过程中，主线程仍然可以直接修改原来的数据
		既保证了快照的完整性，也允许主线程同时对数据进行修改，避免了对正常业务的影响
	总结
		Redis 会使用 bgsave 对当前内存中的所有数据做快照，这个操作是子进程在后台完成的，这就允许主线程同时可以修改数据

可以每秒做一次快照吗？
	快照机制下的数据丢失
		要想尽可能恢复数据，t 值就要尽可能小，t 越小，就越像“连拍”
		t 值可以小到什么程度呢，比如说是不是可以每秒做一次快照？
		毕竟，每次快照都是由 bgsave子进程在后台执行，也不会阻塞主线程
		图示 05.rdb_01.jpg
	“连拍”思路错误的
		虽然 bgsave 执行时不阻塞主线程，但是，如果频繁地执行全量快照，也会带来两方面的开销
		一方面，频繁将全量数据写入磁盘，会给磁盘带来很大压力，多个快照竞争有限的磁盘带宽
			前一个快照还没有做完，后一个又开始做了，容易造成恶性循环
		另一方面，bgsave 子进程需要通过 fork 操作从主线程创建出来
			虽然，子进程在创建后不会再阻塞主线程，但是，fork 这个创建过程本身会阻塞主线程，而且主线程的内存越大，阻塞时间越长
			如果频繁 fork 出 bgsave 子进程，这就会频繁阻塞主线程了
		那么，有什么其他好方法吗？
	增量快照
		做了一次全量快照后，后续的快照只对修改的数据进行快照记录，这样可以避免每次全量快照的开销
		图示 05.rdb_02.jpg
		在第一次做完全量快照后，T1 和 T2 时刻如果再做快照，我们只需要将被修改的数据写入快照文件就行
		但是，这么做的前提是，我们需要记住哪些数据被修改了
		不要小瞧这个“记住”功能，它需要我们使用额外的元数据信息去记录哪些数据被修改了，这会带来额外的空间开销问题
	“记住”功能的开销示例
		如果我们对每一个键值对的修改，都做个记录，那么，如果有 1 万个被修改的键值对，我们就需要有 1 万条额外的记录
		而且，有的时候，键值对非常小，比如只有 32 字节，而记录它被修改的元数据信息，可能就需要 8 字节
		即，为了“记住”修改，引入的额外空间开销比较大
		这对于内存资源宝贵的 Redis 来说，有些得不偿失
	AOF + RDB
		诉求：既能利用 RDB 的快速恢复，又能以较小的开销做到尽量少丢数据
			虽然跟 AOF 相比，快照的恢复速度快，但是，快照的频率不好把握，如果频率太低，两次快照间一旦宕机，就可能有比较多的数据丢失
			如果频率太高，又会产生额外开销
		Redis 4.0 中提出了一个混合使用 AOF 日志和内存快照的方法
			内存快照以一定的频率执行，在两次快照之间，使用 AOF 日志记录这期间的所有命令操作
		好处
			快照不用很频繁地执行，这就避免了频繁 fork 对主线程的影响
			而且，AOF日志也只用记录两次快照间的操作，也就是说，不需要记录所有操作了
			因此，就不会出现文件过大的情况了，也可以避免重写开销
		图示 05.rdb_03.jpg
			T1 和 T2 时刻的修改，用 AOF 日志记录，等到第二次做全量快照时，就可以清空 AOF 日志
			因为此时的修改都已经记录到快照中了，恢复时就不再用日志了
			既能享受到 RDB 文件快速恢复的好处，又能享受到 AOF 只记录操作命令的简单优势

小结
	快速恢复数据库
		只需要把 RDB 文件直接读入内存，这就避免了 AOF 需要顺序、逐一重新执行操作命令带来的低效性能问题
	bgsave 和写时复制方式
		尽可能减少了内存快照对正常读写的影响，但是，频繁快照仍然是不太能接受的
	混合使用 RDB 和 AOF
		正好可以取两者之长，避两者之短，以较小的性能开销保证数据可靠性和性能
	AOF 和 RDB 选择问题
		数据不能丢失时，内存快照和 AOF 的混合使用是一个很好的选择
		如果允许分钟级别的数据丢失，可以只使用 RDB
		如果只用 AOF，优先使用 everysec 的配置选项，因为它在可靠性和性能之间取了一个平衡

思考
	使用一个 2 核 CPU、4GB 内存、500GB 磁盘的云主机运行 Redis，Redis 数据库的数据量大小差不多是 2GB
	使用了 RDB 做持久化保证
	Redis 的运行负载以修改操作为主，写读比例差不多在 8:2 左右，也就是说，如果有 100 个请求，80 个请求执行的是修改操作
	你觉得，在这个场景下，用 RDB 做持久化有什么风险吗？你能帮着一起分析分析吗？



*/
