package _1_basic

/*
数据结构：快速的Redis有哪些慢操作？

Redis 的快，到底是快在哪里呢？
	它接收到一个键值对操作后，能以微秒级别的速度找到数据，并快速完成操作
	原因
		1.它是内存数据库，所有操作都在内存上完成，内存的访问速度本身就很快
		2.归功于它的数据结构
			键值对是按一定的数据结构来组织的，操作键值对最终就是对数据结构进行增删改查操作
			所以高效的数据结构是 Redis 快速处理数据的基础
			a)O(1) 复杂度的哈希表被广泛使用，包括String、Hash 和 Set，它们的操作复杂度基本由哈希表决定
			b)Sorted Set 也采用了 O(logN) 复杂度的跳表
	Redis 数据类型
		String、List、Hash、Set、Sorted Set
	6 种底层数据结构
		简单动态字符串、双向链表、压缩列表、哈希表、跳表、整数数组
	数据类型 & 数据结构
		02.structure_6.jpg
		集合类型：
			List、Hash、Set、Sorted Set
			特点是一个键对应了一个集合的数据
	思考
		这些数据结构都是值的底层实现，键和值本身之间用什么结构组织？
		为什么集合类型有那么多的底层结构，它们都是怎么组织数据的，都很快吗？
		什么是简单动态字符串，和常用的字符串是一回事吗？

键和值用什么结构组织？
	哈希表
		Redis 使用了一个哈希表来保存所有键值对
		一个哈希表，其实就是一个数组，数组的每个元素称为一个哈希桶
			所以常说，一个哈希表是由多个哈希桶组成的，每个哈希桶中保存了键值对数据
	如果值是集合类型的话，作为数组元素的哈希桶怎么来保存呢？
		哈希桶中的元素保存的并不是值本身，而是指向具体值的指针
		不管值是 String，还是集合类型，哈希桶中的元素都是指向它们的指针
	全局哈希表
		图示 02.structure_hash.jpg
			哈希桶中的 entry 元素中保存了*key和*value指针，分别指向了实际的键和值
			这样一来，即使值是一个集合，也可以通过*value指针被查找到
		好处
			可以用 O(1) 的时间复杂度来快速查找到键值对——我们只需要计算键的哈希值，就可以知道它所对应的哈希桶位置
			然后就可以访问相应的 entry 元素
			这个查找过程主要依赖于哈希计算，和数据量的多少并没有直接关系
		潜在风险
			哈希表的冲突问题和 rehash 可能带来的操作阻塞
为什么哈希表操作变慢了？
	哈希冲突
		两个 key 的哈希值和哈希桶计算对应关系时，正好落在了同一个哈希桶中
		毕竟，哈希桶的个数通常要少于 key 的数量，这也就是说，难免会有一些 key 的哈希值对应到了同一个哈希桶中
	链式哈希
		同一个哈希桶中的多个元素用一个链表来保存，它们之间依次用指针连接
			这个链表，也叫做哈希冲突链
		02.structure_hash_list.jpg
	rehash 简介（扩容）
		场景
			如果哈希表里写入的数据越来越多，哈希冲突可能也会越来越多
			这就会导致某些哈希冲突链过长，进而导致这个链上的元素查找耗时长，效率降低
			对于追求“快”的 Redis 来说，这是不太能接受的
		扩容概述
			增加现有的哈希桶数量，让逐渐增多的 entry 元素能在更多的桶之间分散保存，减少单个桶中的元素数量
			从而减少单个桶中的冲突
		扩容思路
			Redis 默认使用了两个全局哈希表：哈希表 1 和哈希表 2
			一开始，当你刚插入数据时，默认使用哈希表 1，此时的哈希表 2 并没有被分配空间
		rehash 过程
			1. 给哈希表 2 分配更大的空间，例如是当前哈希表 1 大小的两倍
			2. 把哈希表 1 中的数据重新映射并拷贝到哈希表 2 中
			3. 释放哈希表 1 的空间
			到此，就可以从哈希表 1 切换到哈希表 2，用增大的哈希表 2 保存更多数据
			而原来的哈希表 1 留作下一次 rehash 扩容备用
		问题
			第二步涉及大量的数据拷贝，如果一次性把哈希表 1 中的数据都迁移完，会造成 Redis 线程阻塞，无法服务其他请求
	渐进式 rehash
		图示 02.structure_rehash.jpg
		拷贝数据期间，Redis 仍然正常处理客户端请求
		每处理一个请求时，从哈希表 1 中的第一个索引位置开始，顺带着将这个索引位置上的所有 entries 拷贝到哈希表 2 中
		等处理下一个请求时，再顺带拷贝哈希表 1 中的下一个索引位置的 entries
		巧妙地把一次性大量拷贝的开销，分摊到了多次处理请求的过程中，避免了耗时操作，保证了数据的快速访问

集合数据操作效率
	集合类型的值
		第一步是通过全局哈希表找到对应的哈希桶位置
		第二步是在集合中再增删改查
	集合的操作效率和哪些因素相关呢？
		首先，与集合的底层数据结构有关
			例如，使用哈希表实现的集合，要比使用链表实现的集合访问效率更高
		其次，操作效率和这些操作本身的执行特点有关
			比如读写一个元素的操作要比读写所有元素的效率高
有哪些底层数据结构？
	整数数组、双向链表、哈希表、压缩列表、跳表
	整数数组和双向链表
		通过数组下标或者链表的指针逐个元素访问，操作复杂度基本是 O(N)，操作效率比较低
	压缩列表（ZipList）
		概述
			压缩列表实际上类似于一个数组
		02.structure_ziplist.jpg
			表头有三个字段 zlbytes、zltail 和 zllen
			zlbytes：列表长度
			zltail：列表尾的偏移量
			zllen：列表中的 entry 个数
			zlend：压缩列表在表尾还有一个 zlend，表示列表结束
		时间复杂度
			查找定位第一个元素和最后一个元素，可以通过表头三个字段的长度直接定位，复杂度是 O(1)
			而查找其他元素时，就没有这么高效了，只能逐个查找，此时的复杂度就是 O(N) 了
	跳表
		概述
			跳表在链表的基础上，增加了多级索引，通过索引位置的几个跳转，实现数据的快速定位
		02.structure_skiplist.jpg
		时间复杂度
			O(log N)
	bigO
		02.structure_bigo.jpg
不同操作的复杂度
	操作类型举例
		读写单个元素：HGET、HSET
		操作多个元素：SADD
		整个集合遍历操作：SMEMBERS
	口诀
		单元素操作是基础
		范围操作非常耗时
		统计操作通常高效
		例外情况只有几个
	单元操作：每一种集合类型对单个数据实现的增删改查操作
		单元素
			例如，Hash 类型的 HGET、HSET 和 HDEL，Set 类型的 SADD、SREM、SRANDMEMBER 等
			这些操作的复杂度由集合采用的数据结构决定
				例如，HGET、HSET 和 HDEL 是对哈希表做操作，所以它们的复杂度都是 O(1)
				Set 类型用哈希表作为底层数据结构时，它的 SADD、SREM、SRANDMEMBER 复杂度也是 O(1)
		多元素：同时进行增删改查
			例如 Hash 类型的 HMGET 和 HMSET，Set 类型的 SADD 也支持同时增加多个元素
			此时，这些操作的复杂度，就是由单个元素操作复杂度和元素个数决定的
				例如，HMSET 增加 M 个元素时，复杂度就从 O(1) 变成 O(M) 了
	范围操作：集合类型中的遍历操作，可以返回集合中的所有数据
		一次性
			比如 Hash 类型的 HGETALL 和 Set 类型的 SMEMBERS
			或者返回一个范围内的部分数据，比如 List 类型的 LRANGE 和 ZSet 类型的 ZRANGE
			这类操作的复杂度一般是 O(N)，比较耗时，我们应该尽量避免
		渐进式
			Redis 从 2.8 版本开始提供了 SCAN 系列操作（包括 HSCAN，SSCAN 和 ZSCAN），这类操作实现了渐进式遍历，每次只返回有限数量的数据
			这样一来，相比于 HGETALL、SMEMBERS 这类操作来说，就避免了一次性返回所有元素而导致的 Redis 阻塞
	统计操作：集合类型对集合中所有元素个数的记录
		例如 LLEN 和 SCARD。这类操作复杂度只有 O(1)
		这是因为当集合类型采用压缩列表、双向链表、整数数组这些数据结构时，这些结构中专门记录了元素的个数统计，因此可以高效地完成相关操作
	例外情况：某些数据结构的特殊记录，例如压缩列表和双向链表都会记录表头和表尾的偏移量
		对于 List 类型的 LPOP、RPOP、LPUSH、RPUSH 这四个操作来说，它们是在列表的头尾增删元素，这就可以通过偏移量直接定位
		所以它们的复杂度也只有 O(1)，可以实现快速操

总结
	哈希表
		Redis 快速操作键值对（之数据结构）
			O(1) 复杂度的哈希表被广泛使用，包括String、Hash 和 Set，它们的操作复杂度基本由哈希表决定
			Sorted Set 也采用了 O(logN) 复杂度的跳表
		用其他命令来替代
			集合类型的范围操作，因为要遍历底层数据结构，复杂度通常是 O(N)
			用其他命令来替代，例如可以用 SCAN 来代替，避免在 Redis 内部产生费时的全集合遍历操作
	List 类型
		双向链表和压缩列表的操作复杂度都是 O(N)
		因地制宜地使用 List 类型
			例如，既然它的 POP/PUSH 效率很高，那么就将它主要用于 FIFO 队列场景，而不是作为一个可以随机读写的集合

思考
	整数数组和压缩列表在查找时间复杂度方面并没有很大的优势，那为什么 Redis 还会把它们作为底层数据结构呢？
*/