package models

func GetResource(resource *Resource) *Resource {
	r := &Resource{Key: resource.Key, Owner: resource.Owner, Value: resource.Value}
	if resource.TypeCode == UNKNOWN {
		r.TypeCode = GetTypeCode(resource.Type)
		r.Type = resource.Type
	} else {
		r.TypeCode = resource.TypeCode
		r.Type = GetType(resource)
	}
	return r
}

func GetTypeCode(lockType string) TypeCode {
	switch lockType {
	case LockType:
		return LOCK
	case PresenceType:
		return PRESENCE
	default:
		return UNKNOWN
	}
}

func GetType(resource *Resource) string {
	switch resource.TypeCode {
	case LOCK:
		return LockType
	case PRESENCE:
		return PresenceType
	default:
		return resource.Type
	}
}
