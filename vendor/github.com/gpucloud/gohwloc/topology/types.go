package topology

// #cgo LDFLAGS: -lhwloc
// #include <hwloc.h>
import "C"
import "github.com/gpucloud/gohwloc/bitmap"

// HwlocNodeSet A node set is a bitmap whose bits are set according to NUMA memory node physical OS indexes.
/*
 * It may be consulted and modified with the bitmap API as any
 * ::hwloc_bitmap_t (see hwloc/bitmap.h).
 * Each bit may be converted into a NUMA node object using
 * hwloc_get_numanode_obj_by_os_index().
 *
 * When binding memory on a system without any NUMA node,
 * the single main memory bank is considered as NUMA node #0.
 *
 * See also \ref hwlocality_helper_nodeset_convert.
 */
type HwlocNodeSet struct {
	bitmap.BitMap
	hwloc_nodeset_t C.hwloc_bitmap_t
}

// HwlocObjType Type of topology object.
// Do not rely on the ordering or completeness of the values as new ones
// may be defined in the future!  If you need to compare types, use
// hwloc_compare_types() instead.
type HwlocObjType int

const (
	// HwlocObjMachine Machine.
	// A set of processors and memory with cache coherency.
	// This type is always used for the root object of a topology, and never used anywhere else.
	// Hence its parent is always NULL.
	HwlocObjMachine HwlocObjType = iota

	// HwlocObjPackage Physical package.
	// The physical package that usually gets inserted
	// into a socket on the motherboard.
	// A processor package usually contains multiple cores.
	HwlocObjPackage
	// HwlocObjCore Core
	// A computation unit (may be shared by several logical processors).
	HwlocObjCore
	// HwlocObjPU Processing Unit, or (Logical) Processor.
	// An execution unit (may share a core with some
	// other logical processors, e.g. in the case of an SMT core).
	// This is the smallest object representing CPU resources,
	// it cannot have any child except Misc objects.
	// Objects of this kind are always reported and can
	// thus be used as fallback when others are not.
	HwlocObjPU
	// HwlocObjL1Cache Level 1 Data (or Unified) Cache.
	HwlocObjL1Cache
	// HwlocObjL2Cache Level 2 Data (or Unified) Cache.
	HwlocObjL2Cache
	// HwlocObjL3Cache Level 3 Data (or Unified) Cache.
	HwlocObjL3Cache
	// HwlocObjL4Cache Level 4 Data (or Unified) Cache.
	HwlocObjL4Cache
	// HwlocObjL5Cache Level 5 Data (or Unified) Cache.
	HwlocObjL5Cache

	// HwlocObjL1ICache  Level 1 instruction Cache (filtered out by default).
	HwlocObjL1ICache
	// HwlocObjL2ICache Level 2 instruction Cache (filtered out by default).
	HwlocObjL2ICache
	// HwlocObjL3ICache Level 3 instruction Cache (filtered out by default).
	HwlocObjL3ICache

	// HwlocObjGroup Group objects.
	// Objects which do not fit in the above but are
	// detected by hwloc and are useful to take into
	// account for affinity. For instance, some operating systems
	// expose their arbitrary processors aggregation this
	// way.  And hwloc may insert such objects to group
	// NUMA nodes according to their distances.
	// See also \ref faq_groups.
	// These objects are removed when they do not bring
	// any structure (see ::HWLOC_TYPE_FILTER_KEEP_STRUCTURE).
	HwlocObjGroup

	// HwlocObjNumaNode NUMA node.
	// An object that contains memory that is directly
	// and byte-accessible to the host processors.
	// It is usually close to some cores (the corresponding objects
	// are descendants of the NUMA node object in the hwloc tree).
	// This is the smallest object representing Memory resources,
	// it cannot have any child except Misc objects.
	// However it may have Memory-side cache parents.
	// There is always at least one such object in the topology
	// even if the machine is not NUMA.
	// Memory objects are not listed in the main children list,
	// but rather in the dedicated Memory children list.
	// NUMA nodes have a special depth ::HWLOC_TYPE_DEPTH_NUMANODE
	// instead of a normal depth just like other objects in the main tree.
	HwlocObjNumaNode

	// HwlocObjBridge Bridge (filtered out by default).
	// Any bridge that connects the host or an I/O bus,
	// to another I/O bus.
	// They are not added to the topology unless I/O discovery
	// is enabled with hwloc_topology_set_flags().
	// I/O objects are not listed in the main children list,
	// but rather in the dedicated io children list.
	// I/O objects have NULL CPU and node sets.
	HwlocObjBridge
	// HwlocObjPCIDevice PCI device (filtered out by default).
	// They are not added to the topology unless I/O discovery
	// is enabled with hwloc_topology_set_flags().
	// I/O objects are not listed in the main children list,
	// but rather in the dedicated io children list.
	// I/O objects have NULL CPU and node sets.
	HwlocObjPCIDevice
	// HwlocObjOSDevice Operating system device (filtered out by default).
	// They are not added to the topology unless I/O discovery
	// is enabled with hwloc_topology_set_flags().
	// I/O objects are not listed in the main children list,
	// but rather in the dedicated io children list.
	// I/O objects have NULL CPU and node sets.
	HwlocObjOSDevice

	// HwlocObjMisc Miscellaneous objects (filtered out by default).
	// Objects without particular meaning, that can e.g. be
	// added by the application for its own use, or by hwloc
	// for miscellaneous objects such as MemoryModule (DIMMs).
	// These objects are not listed in the main children list,
	// but rather in the dedicated misc children list.
	// Misc objects may only have Misc objects as children,
	// and those are in the dedicated misc children list as well.
	// Misc objects have NULL CPU and node sets.
	HwlocObjMisc

	// HwlocObjMemCache Memory-side cache (filtered out by default).
	// A cache in front of a specific NUMA node.
	// This object always has at least one NUMA node as a memory child.
	// Memory objects are not listed in the main children list,
	// but rather in the dedicated Memory children list.
	// Memory-side cache have a special depth ::HWLOC_TYPE_DEPTH_MEMCACHE
	// instead of a normal depth just like other objects in the
	// main tree.
	HwlocObjMemCache
)

type HwlocObjCacheType int

const (
	// HwlocObjCacheUnified Unified cache.
	HwlocObjCacheUnified HwlocObjCacheType = iota
	// HwlocObjCacheData Data cache.
	HwlocObjCacheData
	// HwlocObjCacheInstruction Instruction cache (filtered out by default).
	HwlocObjCacheInstruction
)

type HwlocObjBridgeType int

const (
	// HwlocObjBridgeHost Host-side of a bridge, only possible upstream.
	HwlocObjBridgeHost HwlocObjBridgeType = iota
	// HwlocObjBridgePCI PCI-side of a bridge.
	HwlocObjBridgePCI
)

type HwlocObjOSDevType int

const (
	// HwlocObjOSDevBlock Operating system block device, or non-volatile memory device.
	// For instance "sda" or "dax2.0" on Linux.
	HwlocObjOSDevBlock HwlocObjOSDevType = iota
	// HwlocObjOSDevGPU Operating system GPU device.
	// For instance ":0.0" for a GL display, "card0" for a Linux DRM device.
	HwlocObjOSDevGPU
	// HwlocObjOSDevNetwork Operating system network device.
	// For instance the "eth0" interface on Linux.
	HwlocObjOSDevNetwork
	// HwlocObjOSDevOpenFabrics Operating system openfabrics device.
	// For instance the "mlx4_0" InfiniBand HCA, or "hfi1_0" Omni-Path interface on Linux.
	HwlocObjOSDevOpenFabrics
	// HwlocObjOSDevDMA Operating system dma engine device.
	// For instance the "dma0chan0" DMA channel on Linux.
	HwlocObjOSDevDMA
	// HwlocObjOSDevCoproc Operating system co-processor device.
	// For instance "mic0" for a Xeon Phi (MIC) on Linux, "opencl0d0" for a OpenCL device, "cuda0" for a CUDA device.
	HwlocObjOSDevCoproc
)

// HwlocNumaNodeAttr NUMA node-specific Object Attributes
type HwlocNumaNodeAttr struct {
	LocalMemory     uint64
	PageTypesLength uint
}

// HwlocCacheAttr Cache-specific Object Attributes
type HwlocCacheAttr struct {
	Size          uint64
	Depth         uint
	LineSize      uint
	Associativity int
	Type          HwlocObjCacheType
}

// HwlocGroupAttr Group-specific Object Attribute
type HwlocGroupAttr struct {
	// Depth Depth of group object, It may change if intermediate Group objects are added.
	Depth uint
	// Kind Internally-used kind of group.
	Kind uint
	// SubKind Internally-used subkind to distinguish different levels of groups with same kind.
	SubKind uint
}

// HwlocPCIDevAttr PCI Device specific Object Attributes
type HwlocPCIDevAttr struct {
	Domain      uint16
	Bus         uint8
	Dev         uint8
	Func        uint8
	ClassID     uint16
	VendorID    uint16
	DeviceID    uint16
	SubVendorID uint16
	SubDeviceID uint16
	Revision    uint8
	LinkSpeed   float32 // in GB/s
}

// HwlocBridgeAttr specific Object Attribues
type HwlocBridgeAttr struct {
	UpstreamPCI                 *HwlocPCIDevAttr
	UpstreamType                HwlocObjBridgeType
	DownStreamPCIDomain         uint
	DownStreamPCISecondaryBus   string
	DownStreamPCISubordinateBus string
	DownStreamType              HwlocObjBridgeType
	Depth                       uint
}

// HwlocObjAttr Object type-specific Attributes
type HwlocObjAttr struct {
	NumaNode  *HwlocNumaNodeAttr
	Cache     *HwlocCacheAttr
	Group     *HwlocGroupAttr
	PCIDev    *HwlocPCIDevAttr
	Bridge    *HwlocBridgeAttr
	OSDevType HwlocObjOSDevType
}

// HwlocObject Structure of a topology object
type HwlocObject struct {
	// HwlocObjType Type of object
	Type HwlocObjType
	// Subtype string to better describe the type field
	SubType string
	// OSIndex OS-provided physical index number.
	// It is not guaranteed unique across the entire machine, except for PUs and NUMA nodes.
	// Set to HWLOC_UNKNOWN_INDEX if unknown or irrelevant for this object.
	OSIndex uint
	// Name Object-specific name if any.
	// Mostly used for identifying OS devices and Misc objects where
	// a name string is more useful than numerical indexes.
	Name string
	// TotalMemory Total memory (in bytes) in NUMA nodes below this object.
	TotalMemory uint64
	// Attributes Object type-specific Attributes, may be NULL if no attribute value was found global position.
	Attributes *HwlocObjAttr
	// Depth Vertical index in the hierarchy.
	// For normal objects, this is the depth of the horizontal level
	// that contains this object and its cousins of the same type.
	// If the topology is symmetric, this is equal to the parent depth
	// plus one, and also equal to the number of parent/child links
	// from the root object to here.
	// For special objects (NUMA nodes, I/O and Misc) that are not
	// in the main tree, this is a special negative value that
	// corresponds to their dedicated level,
	// see hwloc_get_type_depth() and ::hwloc_get_type_depth_e.
	// Those special values can be passed to hwloc functions such
	// hwloc_get_nbobjs_by_depth() as usual.
	Depth int
	// LogicalIndex Horizontal index in the whole list of similar objects,
	// hence guaranteed unique across the entire machine.
	// Could be a "cousin_rank" since it's the rank within the "cousin" list below
	// Note that this index may change when restricting the topology
	// or when inserting a group.
	LogicalIndex uint
	// NextCousin Next object of same type and depth
	NextCousin *HwlocObject
	// PrevCousin Previous object of same type and depth
	PrevCousin  *HwlocObject
	Parent      *HwlocObject
	SiblingRank uint
	NextSibling *HwlocObject
	PrevSibling *HwlocObject
	// Arity Number of normal children.
	// Memory, Misc and I/O children are not listed here
	// but rather in their dedicated children list.
	Arity uint

	Children   []*HwlocObject
	FirstChild *HwlocObject
	LastChild  *HwlocObject
	// Set if the subtree of normal objects below this object is symmetric,
	// which means all normal children and their children have identical subtrees.
	// Memory, I/O and Misc children are ignored.
	// If set in the topology root object, lstopo may export the topology as a synthetic string.
	SymmetricSubTree int

	MemoryArity      uint
	MemoryFirstChild *HwlocObject
	IOArity          uint
	IOFirstChild     *HwlocObject
	MiscArity        uint
	MiscFirstChild   *HwlocObject
	CPUSet           *HwlocCPUSet
	CompleteCPUSet   *HwlocCPUSet
	NodeSet          *HwlocNodeSet
	CompleteNodeSet  *HwlocNodeSet
	// Infos Array of stringified info type=name.
	Infos map[string]string

	// misc

	// UserData Application-given private data pointer,
	// initialized to \c NULL, use it as you wish.
	UserData []byte

	private C.hwloc_obj_t
}

type HwlocPid uintptr

// HwlocCPUBindFlag Process/Thread binding flags.
/*
 * These bit flags can be used to refine the binding policy.
 *
 * The default (0) is to bind the current process, assumed to be
 * single-threaded, in a non-strict way.  This is the most portable
 * way to bind as all operating systems usually provide it.
 *
 * \note Not all systems support all kinds of binding.  See the
 * "Detailed Description" section of \ref hwlocality_cpubinding for a
 * description of errors that can occur.
 */
type HwlocCPUBindFlag uint8

const (
	// HwlocCPUBindProcess Bind all threads of the current (possibly) multithreaded process.
	HwlocCPUBindProcess HwlocCPUBindFlag = 1 << 0
	// HwlocCPUBindThread Bind current thread of current process.
	HwlocCPUBindThread HwlocCPUBindFlag = 1 << 1
	// HwlocCPUBindStrict Request for strict binding from the OS.
	/* By default, when the designated CPUs are all busy while other
	 * CPUs are idle, operating systems may execute the thread/process
	 * on those other CPUs instead of the designated CPUs, to let them
	 * progress anyway.  Strict binding means that the thread/process
	 * will _never_ execute on other cpus than the designated CPUs, even
	 * when those are busy with other tasks and other CPUs are idle.
	 *
	 * \note Depending on the operating system, strict binding may not
	 * be possible (e.g., the OS does not implement it) or not allowed
	 * (e.g., for an administrative reasons), and the function will fail
	 * in that case.
	 *
	 * When retrieving the binding of a process, this flag checks
	 * whether all its threads  actually have the same binding. If the
	 * flag is not given, the binding of each thread will be
	 * accumulated.
	 *
	 * \note This flag is meaningless when retrieving the binding of a
	 * thread.
	 */
	HwlocCPUBindStrict HwlocCPUBindFlag = 1 << 2

	// HwlocCPUBindNomemBind Avoid any effect on memory binding
	/* On some operating systems, some CPU binding function would also
	 * bind the memory on the corresponding NUMA node.  It is often not
	 * a problem for the application, but if it is, setting this flag
	 * will make hwloc avoid using OS functions that would also bind
	 * memory.  This will however reduce the support of CPU bindings,
	 * i.e. potentially return -1 with errno set to ENOSYS in some
	 * cases.
	 *
	 * This flag is only meaningful when used with functions that set
	 * the CPU binding.  It is ignored when used with functions that get
	 * CPU binding information.
	 * \hideinitializer
	 */
	HwlocCPUBindNomemBind HwlocCPUBindFlag = 1 << 3
)

// HwlocMemBindPolicy Memory binding policy.
/*
 * These constants can be used to choose the binding policy.  Only one policy can
 * be used at a time (i.e., the values cannot be OR'ed together).
 *
 * Not all systems support all kinds of binding.
 * hwloc_topology_get_support() may be used to query about the actual memory
 * binding policy support in the currently used operating system.
 * See the "Detailed Description" section of \ref hwlocality_membinding
 * for a description of errors that can occur.
 */
type HwlocMemBindPolicy int

const (
	// HwlocMemBindDefault Reset the memory allocation policy to the system default.
	/* Depending on the operating system, this may correspond to
	 * ::HWLOC_MEMBIND_FIRSTTOUCH (Linux),
	 * or ::HWLOC_MEMBIND_BIND (AIX, HP-UX, Solaris, Windows).
	 * This policy is never returned by get membind functions.
	 * The nodeset argument is ignored.
	 */
	HwlocMemBindDefault HwlocMemBindPolicy = 0
	// HwlocMemBindFirstTouch Allocate each memory page individually on the local NUMA node of the thread that touches it.
	/*
	 * The given nodeset should usually be hwloc_topology_get_topology_nodeset()
	 * so that the touching thread may run and allocate on any node in the system.
	 *
	 * On AIX, if the nodeset is smaller, pages are allocated locally (if the local
	 * node is in the nodeset) or from a random non-local node (otherwise).
	 */
	HwlocMemBindFirstTouch HwlocMemBindPolicy = 1
	// HwlocMemBindBind Allocate memory on the specified nodes.
	HwlocMemBindBind HwlocMemBindPolicy = 1
	// HwlocMemBindInterleave Allocate memory on the given nodes in an interleaved round-robin manner
	/*The precise layout of the memory across
	 * multiple NUMA nodes is OS/system specific. Interleaving can be
	 * useful when threads distributed across the specified NUMA nodes
	 * will all be accessing the whole memory range concurrently, since
	 * the interleave will then balance the memory references.
	 */
	HwlocMemBindInterleave HwlocMemBindPolicy = 1
	// HwlocMemBindNextTouch For each page bound with this policy, by next time
	// it is touched (and next time only), it is moved from its current
	// location to the local NUMA node of the thread where the memory
	// reference occurred (if it needs to be moved at all).
	HwlocMemBindNextTouch HwlocMemBindPolicy = 1
	// HwlocMemBindMixed Returned by get_membind() functions when multiple
	/* threads or parts of a memory area have differing memory binding
	 * policies.
	 * Also returned when binding is unknown because binding hooks are empty
	 * when the topology is loaded from XML without HWLOC_THISSYSTEM=1, etc.
	 */
	HwlocMemBindMixed HwlocMemBindPolicy = -1
)

// HwlocMemBindFlag Memory binding flags.
/*
 * These flags can be used to refine the binding policy.
 * All flags can be logically OR'ed together with the exception of
 * ::HWLOC_MEMBIND_PROCESS and ::HWLOC_MEMBIND_THREAD;
 * these two flags are mutually exclusive.
 *
 * Not all systems support all kinds of binding.
 * hwloc_topology_get_support() may be used to query about the actual memory
 * binding support in the currently used operating system.
 * See the "Detailed Description" section of \ref hwlocality_membinding
 * for a description of errors that can occur.
 */
type HwlocMemBindFlag uint8

const (
	// HwlocMemBindProcess Set policy for all threads of the specified (possibly
	// multithreaded) process.  This flag is mutually exclusive with ::HWLOC_MEMBIND_THREAD.
	HwlocMemBindProcess HwlocMemBindFlag = 1 << 0
	// HwlocMemBindThread Set policy for a specific thread of the current process.
	// This flag is mutually exclusive with ::HWLOC_MEMBIND_PROCESS.
	HwlocMemBindThread HwlocMemBindFlag = 1 << 1
	// HwlocMemBindStrict  Request strict binding from the OS.
	/* The function will fail if the binding can not be guaranteed / completely enforced.
	 *
	 * This flag has slightly different meanings depending on which
	 * function it is used with.
	 */
	HwlocMemBindStrict HwlocMemBindFlag = 1 << 2
	// HwlocMemBindMigrate Migrate existing allocated memory.
	/* If the memory cannot
	 * be migrated and the ::HWLOC_MEMBIND_STRICT flag is passed, an error
	 * will be returned.
	 */
	HwlocMemBindMigrate HwlocMemBindFlag = 1 << 3
	// HwlocMemBindNoCPUBind Avoid any effect on CPU binding.
	/*
	 * On some operating systems, some underlying memory binding
	 * functions also bind the application to the corresponding CPU(s).
	 * Using this flag will cause hwloc to avoid using OS functions that
	 * could potentially affect CPU bindings.  Note, however, that using
	 * NOCPUBIND may reduce hwloc's overall memory binding
	 * support. Specifically: some of hwloc's memory binding functions
	 * may fail with errno set to ENOSYS when used with NOCPUBIND.
	 */
	HwlocMemBindNoCPUBind HwlocMemBindFlag = 1 << 4
	// HwlocMemBindByNodeSet Consider the bitmap argument as a nodeset.
	/*
	 * The bitmap argument is considered a nodeset if this flag is given,
	 * or a cpuset otherwise by default.
	 *
	 * Memory binding by CPU set cannot work for CPU-less NUMA memory nodes.
	 * Binding by nodeset should therefore be preferred whenever possible.
	 */
	HwlocMemBindByNodeSet HwlocMemBindFlag = 1 << 5
)

// HwlocTopologyFlags Flags to be set onto a topology context before load.
/*
 * Flags should be given to hwloc_topology_set_flags().
 * They may also be returned by hwloc_topology_get_flags().
 */
type HwlocTopologyFlags uint64

var defaultFlag uint64 = 1
var (
	// HwlocTopologyFlagIncludeDisallowed Detect the whole system, ignore reservations, include disallowed objects.
	/*
	 * Gather all resources, even if some were disabled by the administrator.
	 * For instance, ignore Linux Cgroup/Cpusets and gather all processors and memory nodes.
	 *
	 * When this flag is not set, PUs and NUMA nodes that are disallowed are not added to the topology.
	 * Parent objects (package, core, cache, etc.) are added only if some of their children are allowed.
	 * All existing PUs and NUMA nodes in the topology are allowed.
	 * hwloc_topology_get_allowed_cpuset() and hwloc_topology_get_allowed_nodeset()
	 * are equal to the root object cpuset and nodeset.
	 *
	 * When this flag is set, the actual sets of allowed PUs and NUMA nodes are given
	 * by hwloc_topology_get_allowed_cpuset() and hwloc_topology_get_allowed_nodeset().
	 * They may be smaller than the root object cpuset and nodeset.
	 *
	 * If the current topology is exported to XML and reimported later, this flag
	 * should be set again in the reimported topology so that disallowed resources
	 * are reimported as well.
	 */
	HwlocTopologyFlagIncludeDisallowed = HwlocTopologyFlags(uint64(1) << 0)
	// HwlocTopologyFlagIsThisSystem Assume that the selected backend provides the topology for the system on which we are running.
	/*
	 * This forces hwloc_topology_is_thissystem() to return 1, i.e. makes hwloc assume that
	 * the selected backend provides the topology for the system on which we are running,
	 * even if it is not the OS-specific backend but the XML backend for instance.
	 * This means making the binding functions actually call the OS-specific
	 * system calls and really do binding, while the XML backend would otherwise
	 * provide empty hooks just returning success.
	 *
	 * Setting the environment variable HWLOC_THISSYSTEM may also result in the
	 * same behavior.
	 *
	 * This can be used for efficiency reasons to first detect the topology once,
	 * save it to an XML file, and quickly reload it later through the XML
	 * backend, but still having binding functions actually do bind.
	 */
	HwlocTopologyFlagIsThisSystem = HwlocTopologyFlags(uint64(1) << 1)
	// HwlocTopologyFlagThisSystemAllowedResources Get the set of allowed resources from the local operating system even if the topology was loaded from XML or synthetic description.
	/*
	 * If the topology was loaded from XML or from a synthetic string,
	 * restrict it by applying the current process restrictions such as
	 * Linux Cgroup/Cpuset.
	 *
	 * This is useful when the topology is not loaded directly from
	 * the local machine (e.g. for performance reason) and it comes
	 * with all resources, while the running process is restricted
	 * to only parts of the machine.
	 *
	 * This flag is ignored unless ::HWLOC_TOPOLOGY_FLAG_IS_THISSYSTEM is
	 * also set since the loaded topology must match the underlying machine
	 * where restrictions will be gathered from.
	 *
	 * Setting the environment variable HWLOC_THISSYSTEM_ALLOWED_RESOURCES
	 * would result in the same behavior.
	 */
	HwlocTopologyFlagThisSystemAllowedResources = HwlocTopologyFlags(uint64(1) << 2)
)

// HwlocTopologyDiscoverySupport Flags describing actual discovery support for this topology.
type HwlocTopologyDiscoverySupport struct {
	// Detecting the number of PU objects is supported.
	PU uint8
	// Detecting the number of NUMA nodes is supported.
	Numa uint8
	// Detecting the amount of memory in NUMA nodes is supported.
	NumaMemory uint8
	// Detecting and identifying PU objects that are not available to the current process is supported.
	DisallowedPU uint8
	// Detecting and identifying NUMA nodes that are not available to the current process is supported.
	DisallowedNuma uint8
}

// HwlocTopologyCPUBindSupport Flags describing actual PU binding support for this topology.
// A flag may be set even if the feature isn't supported in all cases
// (e.g. binding to random sets of non-contiguous objects).
type HwlocTopologyCPUBindSupport struct {
	/** Binding the whole current process is supported.  */
	SetThisProcCPUBind uint8
	/** Getting the binding of the whole current process is supported.  */
	GetThisProcCPUBind uint8
	/** Binding a whole given process is supported.  */
	SetProcCPUBind uint8
	/** Getting the binding of a whole given process is supported.  */
	GetProcCPUBind uint8
	/** Binding the current thread only is supported.  */
	SetThisThreadCPUBind uint8
	/** Getting the binding of the current thread only is supported.  */
	GetThisThreadCPUBind uint8
	/** Binding a given thread only is supported.  */
	SetThreadCPUBind uint8
	/** Getting the binding of a given thread only is supported.  */
	GetThreadCPUBind uint8
	/** Getting the last processors where the whole current process ran is supported */
	GetThisProcLastCPULocation uint8
	/** Getting the last processors where a whole process ran is supported */
	GetProcLastCPULocation uint8
	/** Getting the last processors where the current thread ran is supported */
	GetThisThreadLastCPULocation uint8
}

// HwlocTopologyMemBindSupport Flags describing actual memory binding support for this topology.
// A flag may be set even if the feature isn't supported in all cases
// (e.g. binding to random sets of non-contiguous objects).
type HwlocTopologyMemBindSupport struct {
}

// HwlocTopologySupport Set of flags describing actual support for this topology.
// This is retrieved with hwloc_topology_get_support() and will be valid until
// the topology object is destroyed.  Note: the values are correct only after discovery.
type HwlocTopologySupport struct {
	discovery *HwlocTopologyDiscoverySupport
	cpubind   *HwlocTopologyCPUBindSupport
	membind   *HwlocTopologyMemBindSupport
}

// HwlocTypeFilter Type filtering flags.
// By default, most objects are kept (::HWLOC_TYPE_FILTER_KEEP_ALL).
// Instruction caches, I/O and Misc objects are ignored by default (::HWLOC_TYPE_FILTER_KEEP_NONE).
// Group levels are ignored unless they bring structure (::HWLOC_TYPE_FILTER_KEEP_STRUCTURE).
// Note that group objects are also ignored individually (without the entire level)
// when they do not bring structure.
type HwlocTypeFilter int

const (
	// HwlocTypeFilterKeepAll Keep all objects of this type.
	// Cannot be set for ::HWLOC_OBJ_GROUP (groups are designed only to add more structure to the topology).
	HwlocTypeFilterKeepAll HwlocTypeFilter = C.HWLOC_TYPE_FILTER_KEEP_ALL
	// HwlocTypeFilterKeepNone gnore all objects of this type.
	// The bottom-level type ::HWLOC_OBJ_PU, the ::HWLOC_OBJ_NUMANODE type, and
	// the top-level type ::HWLOC_OBJ_MACHINE may not be ignored.
	HwlocTypeFilterKeepNone HwlocTypeFilter = C.HWLOC_TYPE_FILTER_KEEP_NONE
	// HwlocTypeFilterKeepStructure nly ignore objects if their entire level does not bring any structure.
	// Keep the entire level of objects if at least one of these objects adds
	// structure to the topology. An object brings structure when it has multiple
	// children and it is not the only child of its parent.
	// If all objects in the level are the only child of their parent, and if none
	// of them has multiple children, the entire level is removed.
	// Cannot be set for I/O and Misc objects since the topology structure does not matter there.
	HwlocTypeFilterKeepStructure HwlocTypeFilter = C.HWLOC_TYPE_FILTER_KEEP_STRUCTURE
	// HwlocTypeFilterKeepImportant Only keep likely-important objects of the given type.
	// It is only useful for I/O object types.
	// For ::HWLOC_OBJ_PCI_DEVICE and ::HWLOC_OBJ_OS_DEVICE, it means that only objects
	// of major/common kinds are kept (storage, network, OpenFabrics, Intel MICs, CUDA,
	// OpenCL, NVML, and displays).
	// Also, only OS devices directly attached on PCI (e.g. no USB) are reported.
	// For ::HWLOC_OBJ_BRIDGE, it means that bridges are kept only if they have children.
	// This flag equivalent to ::HWLOC_TYPE_FILTER_KEEP_ALL for Normal, Memory and Misc types
	// since they are likely important.
	HwlocTypeFilterKeepImportant HwlocTypeFilter = C.HWLOC_TYPE_FILTER_KEEP_IMPORTANT
)

// HwlocRestrictFlags Flags to be given to hwloc_topology_restrict().
type HwlocRestrictFlags int

const (
	// HwlocRestrictFlagRemoveCPULess Remove all objects that became CPU-less.
	// By default, only objects that contain no PU and no memory are removed.
	HwlocRestrictFlagRemoveCPULess HwlocRestrictFlags = C.HWLOC_RESTRICT_FLAG_REMOVE_CPULESS
	// HwlocRestrictFlagByNodeSet Restrict by nodeset instead of CPU set.
	// Only keep objects whose nodeset is included or partially included in the given set.
	// This flag may not be used with ::HWLOC_RESTRICT_FLAG_BYNODESET.
	HwlocRestrictFlagByNodeSet HwlocRestrictFlags = C.HWLOC_RESTRICT_FLAG_BYNODESET
	// HwlocRestrictFlagRemoveMemLess Remove all objects that became Memory-less.
	// By default, only objects that contain no PU and no memory are removed.
	// This flag may only be used with ::HWLOC_RESTRICT_FLAG_BYNODESET.
	HwlocRestrictFlagRemoveMemLess HwlocRestrictFlags = C.HWLOC_RESTRICT_FLAG_REMOVE_MEMLESS
	// HwlocRestrictFlagAdaptMisc Move Misc objects to ancestors if their parents are removed during restriction.
	// If this flag is not set, Misc objects are removed when their parents are removed.
	HwlocRestrictFlagAdaptMisc HwlocRestrictFlags = C.HWLOC_RESTRICT_FLAG_ADAPT_MISC
	// HwlocRestrictFlagAdaptIO Move I/O objects to ancestors if their parents are removed during restriction.
	// If this flag is not set, I/O devices and bridges are removed when their parents are removed.
	HwlocRestrictFlagAdaptIO HwlocRestrictFlags = C.HWLOC_RESTRICT_FLAG_ADAPT_IO
)

// HwlocAllowFlags Flags to be given to hwloc_topology_allow().
type HwlocAllowFlags int

const (
	// HwlocAllowFlagAll Mark all objects as allowed in the topology.
	//  cpuset and  nođeset given to hwloc_topology_allow() must be NULL
	HwlocAllowFlagAll HwlocAllowFlags = C.HWLOC_ALLOW_FLAG_ALL
	// HwlocAllowFlagLocalRestrictions Only allow objects that are available to the current process.
	// The topology must have ::HWLOC_TOPOLOGY_FLAG_IS_THISSYSTEM so that the set
	// of available resources can actually be retrieved from the operating system.
	// cpuset and nođeset given to hwloc_topology_allow() must be NULL.
	HwlocAllowFlagLocalRestrictions HwlocAllowFlags = C.HWLOC_ALLOW_FLAG_LOCAL_RESTRICTIONS
	// HwlocAllowFlagCustom Allow a custom set of objects, given to hwloc_topology_allow() as cpuset and/or nodeset parameters.
	HwlocAllowFlagCustom HwlocAllowFlags = C.HWLOC_ALLOW_FLAG_CUSTOM
)
