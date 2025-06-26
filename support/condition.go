package support

type ConditionType int

const (
	OR              ConditionType = iota + 1 // OR条件
	AND                                      // AND条件
	NOT                                      // NOT条件
	IN                                       // IN条件
	NOT_IN                                   // NOT IN条件
	EQ                                       // 等于条件
	NE                                       // 不等于条件
	GT                                       // 大于条件
	LT                                       // 小于条件
	GTE                                      // 大于等于条件
	LTE                                      // 小于等于条件
	LIKE                                     // LIKE条件
	NOT_LIKE                                 // NOT LIKE条件
	BETWEEN                                  // BETWEEN条件
	NOT_BETWEEN                              // NOT BETWEEN条件
	EXISTS                                   // EXISTS条件
	NOT_EXISTS                               // NOT EXISTS条件
	IN_SUBQUERY                              // IN子查询条件
	IN_VALUES                                // IN值列表条件
	NOT_IN_SUBQUERY                          // NOT IN子查询条件
	NOT_IN_VALUES                            // NOT IN値列表条件
	LIMIT                                    // LIMIT条件
	CUSTOM                                   // 自定义条件
	UNKNOWN                                  // 未知条件类型
)

// Condition 查询条件结构体
type Condition struct {
	Type  ConditionType
	Args  []any
	Left  any
	Right any
}
