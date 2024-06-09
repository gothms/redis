package _2_practice

/*
如何在Redis中保存时间序列数据？

需求
	互联网产品记录用户在网站或者 App 上的点击行为数据，来分析用户行为
	这里的数据一般包括用户 ID、行为类型（例如浏览、登录、下单等）、行为发生的时间戳
		UserID, Type, TimeStamp
	如周期性地统计近万台设备的实时状态，包括设备 ID、压力、温度、湿度，以及对应的时间戳
		DeviceID, Pressure, Temperature, Humidity, TimeStamp
	这些与发生时间相关的一组数据，就是时间序列数据
分析
	这些数据的特点是没有严格的关系模型，记录的信息可以表示成键和值的关系（例如，一个设备 ID 对应一条记录）
	所以，并不需要专门用关系型数据库（例如 MySQL）来保存。而 Redis 的键值数据模型，正好可以满足这里的数据存取需求
	Redis 基于自身数据结构以及扩展模块，提供了两种解决方案

时间序列数据的读写特点
	写
		在实际应用中，时间序列数据通常是持续高并发写入的，例如，需要连续记录数万个设备的实时状态值
		同时，时间序列数据的写入主要就是插入新数据，而不是更新一个已存在的数据
		也就是说，一个时间序列数据被记录后通常就不会变了，因为它就代表了一个设备在某个时刻的状态值
		所以，这种数据的写入特点很简单，就是插入数据快，这就要求我们选择的数据类型，在进行数据插入时，复杂度要低，尽量不要阻塞
		Redis 的 String、Hash 类型插入复杂度是 O(1)，但 String 类型记录小数据时，元数据的内存开销比较大，不太适合保存大量数据
	读
		既有对单条记录的查询，也有对某个时间范围内的数据的查询
		还有一些更复杂的查询，比如对某个时间范围内的数据做聚合计算，对符合查询条件的所有数据做计算，包括计算均值、最大 / 最小值、求和等
		概括时间序列数据的“读”，就是查询模式多
	分析
		针对时间序列数据的“写要快”，Redis 的高性能写特性直接就可以满足了
		而针对“查询模式多”，也就是要支持单点查询、范围查询和聚合计算，Redis 提供了保存时间序列数据的两种方案
		分别可以基于 Hash 和 Sorted Set 实现，以及基于 RedisTimeSeries 模块实现
基于 Hash 和 Sorted Set 保存时间序列数据
	Hash 和 Sorted Set 组合的方式有一个明显的好处：
		它们是 Redis 内在的数据类型，代码成熟和性能稳定。所以，基于这两个数据类型保存时间序列数据，系统稳定性是可以预期的
	问题一：为什么保存时间序列数据，要同时使用这两种类型？
		Hash 类型，满足时间序列数据的单键查询需求
			可以把时间戳作为 Hash 集合的 key，把记录的设备状态值作为 Hash 集合的 value
			图示：
				查询某个时间点或者是多个时间点上的温度数据时，直接使用 HGET 命令或者 HMGET 命令
				可以分别获得 Hash 集合中的一个 key 和多个 key 的 value 值
			短板
				它并不支持对数据进行范围查询
				如果要对 Hash 类型进行范围查询的话，就需要扫描 Hash 集合中的所有数据，再把这些数据取回到客户端进行排序
		Sorted Set 类型，支持按时间戳范围的查询
			它能够根据元素的权重分数来排序
			可以把时间戳作为 Sorted Set 集合的元素分数，把时间点上记录的数据作为元素本身
			图示：
			可以使用 ZRANGEBYSCORE 命令，按照输入的最大时间戳和最小时间戳来查询这个时间范围内的温度值
	问题二：如何保证写入 Hash 和 Sorted Set 是一个原子性的操作呢？
		只有保证了写操作的原子性，才能保证同一个时间序列数据，在 Hash 和 Sorted Set 中，要么都保存了，要么都没保存
		否则，就可能出现 Hash 集合中有时间序列数据，而 Sorted Set 中没有，那么，在进行范围查询时，就没有办法满足查询需求了
		Redis 用来实现简单的事务的 MULTI 和 EXEC 命令
	当多个命令及其参数本身无误时，MULTI 和 EXEC 命令可以保证执行这些命令时的原子性
		MULTI 命令：表示一系列原子性操作的开始
			收到这个命令后，Redis 接下来再收到的命令需要放到一个内部队列中，后续一起执行，保证原子性
		EXEC 命令：表示一系列原子性操作的结束
			一旦 Redis 收到了这个命令，就表示所有要保证原子性的命令操作都已经发送完成了
			此时，Redis 开始执行刚才放到内部队列中的所有命令操作
		图示：
			命令 1 到命令 N 是在 MULTI 命令后、EXEC 命令前发送的，它们会被一起执行，保证原子性
	示例
		把设备在 2020 年 8 月 3 日 9 时 5 分的温度，分别用 HSET 命令和 ZADD 命令写入 Hash 集合和 Sorted Set 集合
			127.0.0.1:6379> MULTI
			OK
			127.0.0.1:6379> HSET device:temperature 202008030911 26.8
			QUEUED
			127.0.0.1:6379> ZADD device:temperature 202008030911 26.8
			QUEUED
			127.0.0.1:6379> EXEC
			1) (integer) 1
			2) (integer) 1
		释义
			首先，Redis 收到了客户端执行的 MULTI 命令
			然后，客户端再执行 HSET 和 ZADD 命令后，Redis 返回的结果为“QUEUED”，表示这两个命令暂时入队，先不执行
			执行了 EXEC 命令后，HSET 命令和 ZADD 命令才真正执行，并返回成功结果（结果值为 1）
	问题三：如何对时间序列数据进行聚合计算？
		聚合计算一般被用来周期性地统计时间窗口内的数据汇总状态，在实时监控与预警等场景下会频繁执行
		Slow 方案
			因为 Sorted Set 只支持范围查询，无法直接进行聚合计算，所以，我们只能先把时间范围内的数据取回到客户端，然后在客户端自行完成聚合计算
			这个方法虽然能完成聚合计算，但是会带来一定的潜在风险
			也就是大量数据在 Redis 实例和客户端间频繁传输，这会和其他操作命令竞争网络资源，导致其他操作变慢
		示例：假设需要每 3 分钟统计一下各个设备的温度状态，一旦设备温度超出了设定的阈值，就要进行报警
			每 3 分钟计算一次的所有设备各指标的最大值，每个设备每 15 秒记录一个指标值，1 分钟就会记录 4 个值，3 分钟就会有 12 个值
			如果要统计的设备指标数量有 33 个，所以单个设备每 3 分钟记录的指标数据有将近 400 个（33 * 12 = 396）
			而设备总数量有 1 万台，这样一来，每 3 分钟就有将近 400 万条（396 * 1 万 = 396 万）数据需要在客户端和 Redis 实例间进行传输
			为了避免客户端和 Redis 实例间频繁的大量数据传输，我们可以使用 RedisTimeSeries 来保存时间序列数据
		RedisTimeSeries
			RedisTimeSeries 支持直接在 Redis 实例上进行聚合计算
			每 3 分钟算一次最大值为例
			在 Redis 实例上直接聚合计算，那么对于单个设备的一个指标值来说，每 3 分钟记录的 12 条数据可以聚合计算成一个值
			单个设备每 3 分钟也就只有 33 个聚合值需要传输，1 万台设备也只有 33 万条数据
			数据量大约是在客户端做聚合计算的十分之一，很显然，可以减少大量数据传输对 Redis 实例网络的性能影响
	小结
		如果只需要进行单个时间点查询或是对某个时间范围查询的话，适合使用 Hash 和 Sorted Set 的组合
			它们都是 Redis 的内在数据结构，性能好，稳定性高
		如果我们需要进行大量的聚合计算，同时网络带宽条件不是太好时，Hash 和 Sorted Set 的组合就不太适合了
			此时，使用 RedisTimeSeries 就更加合适一些

基于 RedisTimeSeries 模块保存时间序列数据




























*/
