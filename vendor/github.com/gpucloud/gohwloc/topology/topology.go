package topology

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lhwloc
// #include <hwloc.h>
import "C"
import (
	"errors"
	"unsafe"
)

var NotImplementError = errors.New("not implemented")

type Topology struct {
	hwloc_topology C.hwloc_topology_t
}

func NewTopology() (*Topology, error) {
	var topology C.hwloc_topology_t = &C.struct_hwloc_topology{}
	C.hwloc_topology_init(&topology) // initialization

	return &Topology{
		hwloc_topology: topology,
	}, nil
}

func (t *Topology) Load() error {
	C.hwloc_topology_set_io_types_filter(t.hwloc_topology, C.HWLOC_TYPE_FILTER_KEEP_IMPORTANT)
	C.hwloc_topology_set_icache_types_filter(t.hwloc_topology, C.HWLOC_TYPE_FILTER_KEEP_ALL)
	C.hwloc_topology_load(t.hwloc_topology) // actual detection
	//t.HwlocObject.Depth = int(C.hwloc_topology_get_depth(t.hwloc_topology))
	return nil
}

// Check Run internal checks on a topology structure
// The program aborts if an inconsistency is detected in the given topology.
// This routine is only useful to developers.
// The input topology should have been previously loaded with Load().
func (t *Topology) Check() error {
	C.hwloc_topology_check(t.hwloc_topology)
	return nil
}

// GetDepth Object levels, depths and types
/** \defgroup hwlocality_levels Object levels, depths and types
 * @{
 *
 * Be sure to see the figure in \ref termsanddefs that shows a
 * complete topology tree, including depths, child/sibling/cousin
 * relationships, and an example of an asymmetric topology where one
 * package has fewer caches than its peers.
 *
 * \brief Get the depth of the hierarchical tree of objects.
 *
 * This is the depth of ::HWLOC_OBJ_PU objects plus one.
 *
 * \note NUMA nodes, I/O and Misc objects are ignored when computing
 * the depth of the tree (they are placed on special levels).
 */
func (t *Topology) GetDepth() (int, error) {
	depth := C.hwloc_topology_get_depth(t.hwloc_topology)
	return int(depth), nil
}

// GetTypeDepth Retruns the depth of objects of type
/** \brief Returns the depth of objects of type \p type.
 *
 * If no object of this type is present on the underlying architecture, or if
 * the OS doesn't provide this kind of information, the function returns
 * ::HWLOC_TYPE_DEPTH_UNKNOWN.
 *
 * If type is absent but a similar type is acceptable, see also
 * hwloc_get_type_or_below_depth() and hwloc_get_type_or_above_depth().
 *
 * If ::HWLOC_OBJ_GROUP is given, the function may return ::HWLOC_TYPE_DEPTH_MULTIPLE
 * if multiple levels of Groups exist.
 *
 * If a NUMA node, I/O or Misc object type is given, the function returns a virtual
 * value because these objects are stored in special levels that are not CPU-related.
 * This virtual depth may be passed to other hwloc functions such as
 * hwloc_get_obj_by_depth() but it should not be considered as an actual
 * depth by the application. In particular, it should not be compared with
 * any other object depth or with the entire topology depth.
 * \sa hwloc_get_memory_parents_depth().
 *
 * \sa hwloc_type_sscanf_as_depth() for returning the depth of objects
 * whose type is given as a string.
 */
func (t *Topology) GetTypeDepth(ht HwlocObjType) (int, error) {
	depth := C.hwloc_get_type_depth(t.hwloc_topology, C.hwloc_obj_type_t(ht))
	return int(depth), nil
}

// GetMemoryParentsDepth Return the depth of parents where memory objects are attached.
/*
 * Memory objects have virtual negative depths because they are not part of
 * the main CPU-side hierarchy of objects. This depth should not be compared
 * with other level depths.
 *
 * If all Memory objects are attached to Normal parents at the same depth,
 * this parent depth may be compared to other as usual, for instance
 * for knowing whether NUMA nodes is attached above or below Packages.
 *
 * \return The depth of Normal parents of all memory children
 * if all these parents have the same depth. For instance the depth of
 * the Package level if all NUMA nodes are attached to Package objects.
 *
 * \return ::HWLOC_TYPE_DEPTH_MULTIPLE if Normal parents of all
 * memory children do not have the same depth. For instance if some
 * NUMA nodes are attached to Packages while others are attached to
 * Groups.
 */
func (t *Topology) GetMemoryParentsDepth() (int, error) {
	depth := C.hwloc_get_memory_parents_depth(t.hwloc_topology)
	return int(depth), nil
}

// GetTypeOrBelowDepth Returns the depth of objects of type or below
/*
 * If no object of this type is present on the underlying architecture, the
 * function returns the depth of the first "present" object typically found
 * inside \p type.
 *
 * This function is only meaningful for normal object types.
 * If a memory, I/O or Misc object type is given, the corresponding virtual
 * depth is always returned (see hwloc_get_type_depth()).
 *
 * May return ::HWLOC_TYPE_DEPTH_MULTIPLE for ::HWLOC_OBJ_GROUP just like
 * hwloc_get_type_depth().
 */
func (t *Topology) GetTypeOrBelowDepth(ht HwlocObjType) (int, error) {
	depth := C.hwloc_get_type_or_below_depth(t.hwloc_topology, C.hwloc_obj_type_t(ht))
	return int(depth), nil
}

// GetTypeOrAboveDepth Returns the depth of objects of type or above
/*
 * If no object of this type is present on the underlying architecture, the
 * function returns the depth of the first "present" object typically
 * containing \p type.
 *
 * This function is only meaningful for normal object types.
 * If a memory, I/O or Misc object type is given, the corresponding virtual
 * depth is always returned (see hwloc_get_type_depth()).
 *
 * May return ::HWLOC_TYPE_DEPTH_MULTIPLE for ::HWLOC_OBJ_GROUP just like
 * hwloc_get_type_depth().
 */
func (t *Topology) GetTypeOrAboveDepth(ht HwlocObjType) (int, error) {
	depth := C.hwloc_get_type_or_above_depth(t.hwloc_topology, C.hwloc_obj_type_t(ht))
	return int(depth), nil
}

// GetDepthType Returns the type of objects at depth
// depth should between 0 and hwloc_topology_get_depth()-1.
// return (hwloc_obj_type_t)-1 if depth \p depth does not exist.
func (t *Topology) GetDepthType(depth int) (HwlocObjType, error) {
	hw := C.hwloc_get_depth_type(t.hwloc_topology, C.int(depth))
	return HwlocObjType(hw), nil
}

// GetNbobjsByDepth Returns the width of level at depth.
func (t *Topology) GetNbobjsByDepth(depth int) (uint, error) {
	w := C.hwloc_get_nbobjs_by_depth(t.hwloc_topology, C.int(depth))
	return uint(w), nil
}

// GetNbobjsByType Returns the width of level type
// If no object for that type exists, 0 is returned.
// If there are several levels with objects of that type, -1 is returned.
func (t *Topology) GetNbobjsByType(ht HwlocObjType) (int, error) {
	nbcores := C.hwloc_get_nbobjs_by_type(t.hwloc_topology, C.hwloc_obj_type_t(ht))
	return int(nbcores), nil
}

// GetRootObj Returns the top-object of the topology-tree.
// Its type is ::HWLOC_OBJ_MACHINE.
func (t *Topology) GetRootObj() (*HwlocObject, error) {
	obj := C.hwloc_get_root_obj(t.hwloc_topology)
	return NewHwlocObject(obj)
}

// GetObjByDepth Returns the topology object at logical index idx from depth
func (t *Topology) GetObjByDepth(depth int, idx uint) (*HwlocObject, error) {
	obj := C.hwloc_get_obj_by_depth(t.hwloc_topology, C.int(depth), C.uint(idx))
	return NewHwlocObject(obj)
}

// GetObjByType Returns the topology object at logical index \p idx with type \p type
/*
 * If no object for that type exists, \c NULL is returned.
 * If there are several levels with objects of that type (::HWLOC_OBJ_GROUP),
 * \c NULL is returned and the caller may fallback to hwloc_get_obj_by_depth().
 */
func (t *Topology) GetObjByType(ht HwlocObjType, idx uint) (*HwlocObject, error) {
	obj := C.hwloc_get_obj_by_type(t.hwloc_topology, C.hwloc_obj_type_t(ht), C.uint(idx))
	return NewHwlocObject(obj)
}

func (t *Topology) Destroy() {
	C.hwloc_topology_destroy(t.hwloc_topology)
}

// SetCPUBind Bind current process or thread on cpus given in physical bitmap set.
/*
 * \return -1 with errno set to ENOSYS if the action is not supported
 * \return -1 with errno set to EXDEV if the binding cannot be enforced
 */
func (t *Topology) SetCPUBind(set HwlocCPUSet, flags int) error {
	C.hwloc_set_cpubind(t.hwloc_topology, set.hwloc_cpuset_t(), C.int(flags))
	return nil
}

// GetCPUBind Get current process or thread binding.
/*
 * Writes into \p set the physical cpuset which the process or thread (according to \e
 * flags) was last bound to.
 */
func (t *Topology) GetCPUBind(flags int) (HwlocCPUSet, error) {
	var set = NewCPUSet(nil)
	C.hwloc_get_cpubind(t.hwloc_topology, set.hwloc_cpuset_t(), C.int(flags))
	return *set, nil
}

// SetProcCPUBind Bind a process pid on cpus given in physical bitmap set.
/* \note \p hwloc_pid_t is \p pid_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note As a special case on Linux, if a tid (thread ID) is supplied
 * instead of a pid (process ID) and ::HWLOC_CPUBIND_THREAD is passed in flags,
 * the binding is applied to that specific thread.
 *
 * \note On non-Linux systems, ::HWLOC_CPUBIND_THREAD can not be used in \p flags.
 */
func (t *Topology) SetProcCPUBind(pid HwlocPid, set HwlocCPUSet, flags int) error {
	C.hwloc_set_proc_cpubind(t.hwloc_topology, C.hwloc_pid_t(pid), set.hwloc_cpuset_t(), C.int(flags))
	return nil
}

// GetProcCPUBind Get the current physical binding of process pid.
/*
 * \note \p hwloc_pid_t is \p pid_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note As a special case on Linux, if a tid (thread ID) is supplied
 * instead of a pid (process ID) and HWLOC_CPUBIND_THREAD is passed in flags,
 * the binding for that specific thread is returned.
 *
 * \note On non-Linux systems, HWLOC_CPUBIND_THREAD can not be used in \p flags.
 */
func (t *Topology) GetProcCPUBind(pid HwlocPid, flags int) (HwlocCPUSet, error) {
	var set = NewCPUSet(nil)
	C.hwloc_get_proc_cpubind(t.hwloc_topology, C.hwloc_pid_t(pid), set.hwloc_cpuset_t(), C.int(flags))
	return *set, nil
}

//#ifdef hwloc_thread_t
/** \brief Bind a thread \p thread on cpus given in physical bitmap \p set.
 *
 * \note \p hwloc_thread_t is \p pthread_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note ::HWLOC_CPUBIND_PROCESS can not be used in \p flags.
 */
//HWLOC_DECLSPEC int hwloc_set_thread_cpubind(hwloc_topology_t topology, hwloc_thread_t thread, hwloc_const_cpuset_t set, int flags);
//#endif

//#ifdef hwloc_thread_t
/** \brief Get the current physical binding of thread \p tid.
 *
 * \note \p hwloc_thread_t is \p pthread_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note ::HWLOC_CPUBIND_PROCESS can not be used in \p flags.
 */
//HWLOC_DECLSPEC int hwloc_get_thread_cpubind(hwloc_topology_t topology, hwloc_thread_t thread, hwloc_cpuset_t set, int flags);
//#endif

// GetLastCPULocation Get the last physical CPU where the current process or thread ran.
/*
 * The operating system may move some tasks from one processor
 * to another at any time according to their binding,
 * so this function may return something that is already
 * outdated.
 *
 * flags can include either ::HWLOC_CPUBIND_PROCESS or ::HWLOC_CPUBIND_THREAD to
 * specify whether the query should be for the whole process (union of all CPUs
 * on which all threads are running), or only the current thread. If the
 * process is single-threaded, flags can be set to zero to let hwloc use
 * whichever method is available on the underlying OS.
 */
func (t *Topology) GetLastCPULocation(flags int) (HwlocCPUSet, error) {
	var set = NewCPUSet(nil)
	C.hwloc_get_last_cpu_location(t.hwloc_topology, set.hwloc_cpuset_t(), C.int(flags))
	return *set, nil
}

// GetProcLastCPULocation Get the last physical CPU where a process ran.
/* The operating system may move some tasks from one processor
 * to another at any time according to their binding,
 * so this function may return something that is already
 * outdated.
 *
 * \note \p hwloc_pid_t is \p pid_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note As a special case on Linux, if a tid (thread ID) is supplied
 * instead of a pid (process ID) and ::HWLOC_CPUBIND_THREAD is passed in flags,
 * the last CPU location of that specific thread is returned.
 *
 * \note On non-Linux systems, ::HWLOC_CPUBIND_THREAD can not be used in \p flags.
 */
func (t *Topology) GetProcLastCPULocation(pid HwlocPid, flags int) (HwlocCPUSet, error) {
	var set = NewCPUSet(nil)
	C.hwloc_get_proc_last_cpu_location(t.hwloc_topology, C.hwloc_pid_t(pid), set.hwloc_cpuset_t(), C.int(flags))
	return *set, nil
}

// SetPid Change which process the topology is viewed from.
/*
 * On some systems, processes may have different views of the machine, for
 * instance the set of allowed CPUs. By default, hwloc exposes the view from
 * the current process. Calling hwloc_topology_set_pid() permits to make it
 * expose the topology of the machine from the point of view of another
 * process.
 *
 * \note \p hwloc_pid_t is \p pid_t on Unix platforms,
 * and \p HANDLE on native Windows platforms.
 *
 * \note -1 is returned and errno is set to ENOSYS on platforms that do not
 * support this feature.
 */
func (t *Topology) SetPid(pid HwlocPid) error {
	C.hwloc_topology_set_pid(t.hwloc_topology, C.hwloc_pid_t(pid))
	return nil
}

// SetSynthetic Enable synthetic topology.
/*
 * Gather topology information from the given \p description,
 * a space-separated string of <type:number> describing
 * the object type and arity at each level.
 * All types may be omitted (space-separated string of numbers) so that
 * hwloc chooses all types according to usual topologies.
 * See also the \ref synthetic.
 *
 * Setting the environment variable HWLOC_SYNTHETIC
 * may also result in this behavior.
 *
 * If \p description was properly parsed and describes a valid topology
 * configuration, this function returns 0.
 * Otherwise -1 is returned and errno is set to EINVAL.
 *
 * Note that this function does not actually load topology
 * information; it just tells hwloc where to load it from.  You'll
 * still need to invoke hwloc_topology_load() to actually load the
 * topology information.
 *
 * \note For convenience, this backend provides empty binding hooks which just
 * return success.
 *
 * \note On success, the synthetic component replaces the previously enabled
 * component (if any), but the topology is not actually modified until
 * hwloc_topology_load().
 */
func (t *Topology) SetSynthetic(desc string) error {
	cdesc := C.CString(desc)
	defer C.free(unsafe.Pointer(cdesc))
	C.hwloc_topology_set_synthetic(t.hwloc_topology, cdesc)
	return nil
}

// SetXMLFile Enable XML-file based topology.
/*
 * Gather topology information from the XML file given at \p xmlpath.
 * Setting the environment variable HWLOC_XMLFILE may also result in this behavior.
 * This file may have been generated earlier with hwloc_topology_export_xml() in hwloc/export.h,
 * or lstopo file.xml.
 *
 * Note that this function does not actually load topology
 * information; it just tells hwloc where to load it from.  You'll
 * still need to invoke hwloc_topology_load() to actually load the
 * topology information.
 *
 * \return -1 with errno set to EINVAL on failure to read the XML file.
 *
 * \note See also hwloc_topology_set_userdata_import_callback()
 * for importing application-specific object userdata.
 *
 * \note For convenience, this backend provides empty binding hooks which just
 * return success.  To have hwloc still actually call OS-specific hooks, the
 * ::HWLOC_TOPOLOGY_FLAG_IS_THISSYSTEM has to be set to assert that the loaded
 * file is really the underlying system.
 *
 * \note On success, the XML component replaces the previously enabled
 * component (if any), but the topology is not actually modified until
 * hwloc_topology_load().
 */
func (t *Topology) SetXMLFile(file string) error {
	f := C.CString(file)
	defer C.free(unsafe.Pointer(f))
	C.hwloc_topology_set_xml(t.hwloc_topology, f)
	return nil
}

// SetXMLBuffer Enable XML based topology using a memory buffer (instead of
/* a file, as with hwloc_topology_set_xml()).
 *
 * Gather topology information from the XML memory buffer given at \p
 * buffer and of length \p size.  This buffer may have been filled
 * earlier with hwloc_topology_export_xmlbuffer() in hwloc/export.h.
 *
 * Note that this function does not actually load topology
 * information; it just tells hwloc where to load it from.  You'll
 * still need to invoke hwloc_topology_load() to actually load the
 * topology information.
 *
 * \return -1 with errno set to EINVAL on failure to read the XML buffer.
 *
 * \note See also hwloc_topology_set_userdata_import_callback()
 * for importing application-specific object userdata.
 *
 * \note For convenience, this backend provides empty binding hooks which just
 * return success.  To have hwloc still actually call OS-specific hooks, the
 * ::HWLOC_TOPOLOGY_FLAG_IS_THISSYSTEM has to be set to assert that the loaded
 * file is really the underlying system.
 *
 * \note On success, the XML component replaces the previously enabled
 * component (if any), but the topology is not actually modified until
 * hwloc_topology_load().
 */
func (t *Topology) SetXMLBuffer() {

}

// SetFlags Set OR'ed flags to non-yet-loaded topology.
/*
 * Set a OR'ed set of ::hwloc_topology_flags_e onto a topology that was not yet loaded.
 *
 * If this function is called multiple times, the last invokation will erase
 * and replace the set of flags that was previously set.
 *
 * The flags set in a topology may be retrieved with hwloc_topology_get_flags()
 */
func (t *Topology) SetFlags(flags HwlocTopologyFlags) error {
	C.hwloc_topology_set_flags(t.hwloc_topology, C.ulong(flags))
	return nil
}

// GetFlags Get OR'ed flags of a topology.
/*
 * Get the OR'ed set of ::hwloc_topology_flags_e of a topology.
 *
 * \return the flags previously set with hwloc_topology_set_flags().
 */
func (t *Topology) GetFlags() (HwlocTopologyFlags, error) {
	flags := C.hwloc_topology_get_flags(t.hwloc_topology)
	return HwlocTopologyFlags(flags), nil
}

// IsThisSystem Does the topology context come from this system?
/*
 * return 1 if this topology context was built using the system
 * running this program.
 * return 0 instead (for instance if using another file-system root,
 * a XML topology file, or a synthetic topology).
 */
func (t *Topology) IsThisSystem() (bool, error) {
	res := C.hwloc_topology_is_thissystem(t.hwloc_topology)
	if res == 1 {
		return true, nil
	}
	return false, nil
}

// GetSupport Retrieve the topology support.
// Each flag indicates whether a feature is supported.
// If set to 0, the feature is not supported.
// If set to 1, the feature is supported, but the corresponding
// call may still fail in some corner cases.
// These features are also listed by hwloc-info \--support
func (t *Topology) GetSupport() (*HwlocTopologySupport, error) {
	s := C.hwloc_topology_get_support(t.hwloc_topology)
	return &HwlocTopologySupport{
		discovery: &HwlocTopologyDiscoverySupport{
			PU: uint8(s.discovery.pu),
		},
		// TODO
		cpubind: &HwlocTopologyCPUBindSupport{},
		membind: &HwlocTopologyMemBindSupport{},
	}, nil
}

// SetTypeFilter Set the filtering for the given object type.
func (t *Topology) SetTypeFilter(ot HwlocObjType, f HwlocTypeFilter) error {
	C.hwloc_topology_set_type_filter(t.hwloc_topology, C.hwloc_obj_type_t(ot), C.enum_hwloc_type_filter_e(f))
	return nil
}

// GetTypeFilter Get the current filtering for the given object type.
func (t *Topology) GetTypeFilter(ot HwlocObjType) (HwlocTypeFilter, error) {
	var filter C.enum_hwloc_type_filter_e
	C.hwloc_topology_get_type_filter(t.hwloc_topology, C.hwloc_obj_type_t(ot), &filter)
	return HwlocTypeFilter(filter), nil
}

// SetAllTypeFilter Set the filtering for all object types.
// If some types do not support this filtering, they are silently ignored.
func (t *Topology) SetAllTypeFilter(f HwlocTypeFilter) error {
	C.hwloc_topology_set_all_types_filter(t.hwloc_topology, C.enum_hwloc_type_filter_e(f))
	return nil
}

// SetCacheTypeFilter Set the filtering for all cache object types.
func (t *Topology) SetCacheTypeFilter(f HwlocTypeFilter) error {
	C.hwloc_topology_set_cache_types_filter(t.hwloc_topology, C.enum_hwloc_type_filter_e(f))
	return nil
}

// SetICacheTypeFilter Set the filtering for all instruction cache object types.
func (t *Topology) SetICacheTypeFilter(f HwlocTypeFilter) error {
	C.hwloc_topology_set_icache_types_filter(t.hwloc_topology, C.enum_hwloc_type_filter_e(f))
	return nil
}

// SetIOTypeFilter Set the filtering for all I/O object types.
func (t *Topology) SetIOTypeFilter(f HwlocTypeFilter) error {
	C.hwloc_topology_set_io_types_filter(t.hwloc_topology, C.enum_hwloc_type_filter_e(f))
	return nil
}

// SetUserData Set the topology-specific userdata pointer.
// Each topology may store one application-given private data pointer.
// It is initialized to \c NULL.
// hwloc will never modify it.
// Use it as you wish, after hwloc_topology_init() and until hwloc_topolog_destroy().
// This pointer is not exported to XML.
func (t *Topology) SetUserData(data unsafe.Pointer) error {
	C.hwloc_topology_set_userdata(t.hwloc_topology, data)
	return nil
}

// GetUserData Retrieve the topology-specific userdata pointer.
// Retrieve the application-given private data pointer that was
// previously set with hwloc_topology_set_userdata().
func (t *Topology) GetUserData() (unsafe.Pointer, error) {
	data := C.hwloc_topology_get_userdata(t.hwloc_topology)
	return data, nil
}

// SetRestrict Restrict the topology to the given CPU set or nodeset.
// Topology \p topology is modified so as to remove all objects that
// are not included (or partially included) in the CPU set \p set.
// All objects CPU and node sets are restricted accordingly.
// If ::HWLOC_RESTRICT_FLAG_BYNODESET is passed in \p flags,
// set is considered a nodeset instead of a CPU set.
// flags is a OR'ed set of ::hwloc_restrict_flags_e.
// This call may not be reverted by restricting back to a larger
// set. Once dropped during restriction, objects may not be brought
// back, except by loading another topology with hwloc_topology_load().
// return 0 on success.
// return -1 with errno set to EINVAL if the input set is invalid.
// The topology is not modified in this case.
// return -1 with errno set to ENOMEM on failure to allocate internal data.
// The topology is reinitialized in this case. It should be either
// destroyed with hwloc_topology_destroy() or configured and loaded again.
func (t *Topology) SetRestrict(bitmap BitMap, flags uint32) error {
	C.hwloc_topology_restrict(t.hwloc_topology, bitmap.bm, C.ulong(flags))
	return nil
}

// SetAllow Change the sets of allowed PUs and NUMA nodes in the topology.
// This function only works if the ::HWLOC_TOPOLOGY_FLAG_INCLUDE_DISALLOWED
// was set on the topology. It does not modify any object, it only changes
// the sets returned by hwloc_topology_get_allowed_cpuset() and
// hwloc_topology_get_allowed_nodeset().
// It is notably useful when importing a topology from another process
// running in a different Linux Cgroup.
// flags must be set to one flag among ::hwloc_allow_flags_e.
// Removing objects from a topology should rather be performed with hwloc_topology_restrict().
func (t *Topology) SetAllow(cpuset HwlocCPUSet, nodeset HwlocNodeSet, flags uint32) error {
	return NotImplementError
}

// InsertMiscObject Add a MISC object as a leaf of the topology
/*
 * A new MISC object will be created and inserted into the topology at the
 * position given by parent. It is appended to the list of existing Misc children,
 * without ever adding any intermediate hierarchy level. This is useful for
 * annotating the topology without actually changing the hierarchy.
 *
 * \p name is supposed to be unique across all Misc objects in the topology.
 * It will be duplicated to setup the new object attributes.
 *
 * The new leaf object will not have any \p cpuset.
 *
 * \return the newly-created object
 *
 * \return \c NULL on error.
 *
 * \return \c NULL if Misc objects are filtered-out of the topology (::HWLOC_TYPE_FILTER_KEEP_NONE).
 *
 * \note If \p name contains some non-printable characters, they will
 * be dropped when exporting to XML, see hwloc_topology_export_xml() in hwloc/export.h.
 */
func (t *Topology) InsertMiscObject(parent *HwlocObject, name string) error {
	return NotImplementError
}

// AllocGroupObject Allocate a Group object to insert later with hwloc_topology_insert_group_object().
/*
 * This function returns a new Group object.
 * The caller should (at least) initialize its sets before inserting the object.
 * See hwloc_topology_insert_group_object().
 *
 * The \p subtype object attribute may be set to display something else
 * than "Group" as the type name for this object in lstopo.
 * Custom name/value info pairs may be added with hwloc_obj_add_info() after
 * insertion.
 *
 * The \p kind group attribute should be 0. The \p subkind group attribute may
 * be set to identify multiple Groups of the same level.
 *
 * It is recommended not to set any other object attribute before insertion,
 * since the Group may get discarded during insertion.
 *
 * The object will be destroyed if passed to hwloc_topology_insert_group_object()
 * without any set defined.
 */
func (t *Topology) AllocGroupObject() (*HwlocObject, error) {
	return nil, NotImplementError
}

// InsertGroupObject Add more structure to the topology by adding an intermediate Group
/*
 * The caller should first allocate a new Group object with hwloc_topology_alloc_group_object().
 * Then it must setup at least one of its CPU or node sets to specify
 * the final location of the Group in the topology.
 * Then the object can be passed to this function for actual insertion in the topology.
 *
 * Either the cpuset or nodeset field (or both, if compatible) must be set
 * to a non-empty bitmap. The complete_cpuset or complete_nodeset may be set
 * instead if inserting with respect to the complete topology
 * (including disallowed, offline or unknown objects).
 *
 * It grouping several objects, hwloc_obj_add_other_obj_sets() is an easy way
 * to build the Group sets iteratively.
 *
 * These sets cannot be larger than the current topology, or they would get
 * restricted silently.
 *
 * The core will setup the other sets after actual insertion.
 *
 * \return The inserted object if it was properly inserted.
 *
 * \return An existing object if the Group was discarded because the topology already
 * contained an object at the same location (the Group did not add any locality information).
 * Any name/info key pair set before inserting is appended to the existing object.
 *
 * \return \c NULL if the insertion failed because of conflicting sets in topology tree.
 *
 * \return \c NULL if Group objects are filtered-out of the topology (::HWLOC_TYPE_FILTER_KEEP_NONE).
 *
 * \return \c NULL if the object was discarded because no set was initialized in the Group
 * before insert, or all of them were empty.
 */
func (t *Topology) InsertGroupObject(group *HwlocObject) error {
	return NotImplementError
}
