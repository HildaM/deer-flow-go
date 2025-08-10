package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

// 演示泛型函数
func GenericProcessor[T any](value T, name string) T {
	fmt.Printf("处理 %s: 类型=%T, 值=%v\n", name, value, value)
	return value
}

// 演示泛型结构体
type Container[T any] struct {
	Value T
	Name  string
}

func (c *Container[T]) Process() {
	fmt.Printf("容器 %s 包含类型 %T 的值: %v\n", c.Name, c.Value, c.Value)
}

// 演示类型约束
type Numeric interface {
	int | int64 | float64
}

func Sum[T Numeric](values []T) T {
	var sum T
	for _, v := range values {
		sum += v
	}
	return sum
}

// 演示编译时类型检查
func TypeSafeOperation[T comparable](a, b T) bool {
	return a == b
}

// 演示运行时类型信息
func InspectType[T any](value T) {
	fmt.Printf("=== 类型检查 %T ===\n", value)
	fmt.Printf("反射类型: %v\n", reflect.TypeOf(value))
	fmt.Printf("类型名称: %s\n", reflect.TypeOf(value).Name())
	fmt.Printf("类型大小: %d 字节\n", unsafe.Sizeof(value))
	fmt.Printf("类型种类: %v\n", reflect.TypeOf(value).Kind())
	fmt.Println()
}

// 演示接口与泛型的区别
func InterfaceProcessor(value interface{}) interface{} {
	// 运行时类型断言 - 有性能开销
	switch v := value.(type) {
	case string:
		return "处理字符串: " + v
	case int:
		return fmt.Sprintf("处理整数: %d", v*2)
	default:
		return "未知类型"
	}
}

func main() {
	fmt.Println("=== Go 泛型类型擦除演示 ===")
	fmt.Println()

	// 1. 基本泛型使用
	fmt.Println("1. 基本泛型函数调用:")
	GenericProcessor[string]("Hello", "字符串")
	GenericProcessor[int](42, "整数")
	GenericProcessor[float64](3.14, "浮点数")
	fmt.Println()

	// 2. 泛型结构体
	fmt.Println("2. 泛型结构体:")
	stringContainer := &Container[string]{Value: "测试", Name: "字符串容器"}
	intContainer := &Container[int]{Value: 100, Name: "整数容器"}
	stringContainer.Process()
	intContainer.Process()
	fmt.Println()

	// 3. 类型约束演示
	fmt.Println("3. 类型约束演示:")
	intSlice := []int{1, 2, 3, 4, 5}
	floatSlice := []float64{1.1, 2.2, 3.3}
	fmt.Printf("整数求和: %d\n", Sum(intSlice))
	fmt.Printf("浮点数求和: %.2f\n", Sum(floatSlice))
	fmt.Println()

	// 4. 编译时类型安全
	fmt.Println("4. 编译时类型安全:")
	fmt.Printf("字符串比较: %v\n", TypeSafeOperation("hello", "hello"))
	fmt.Printf("整数比较: %v\n", TypeSafeOperation(42, 42))
	// 下面这行会编译错误 - 不同类型无法比较
	// fmt.Printf("混合比较: %v\n", TypeSafeOperation("hello", 42))
	fmt.Println()

	// 5. 运行时类型信息检查
	fmt.Println("5. 运行时类型信息:")
	InspectType("字符串")
	InspectType(42)
	InspectType(3.14)
	InspectType([]int{1, 2, 3})

	// 6. 接口 vs 泛型对比
	fmt.Println("6. 接口 vs 泛型对比:")
	
	// 泛型版本 - 编译时确定类型
	genericResult := GenericProcessor("泛型测试", "泛型")
	fmt.Printf("泛型结果: %s (类型: %T)\n", genericResult, genericResult)
	
	// 接口版本 - 运行时类型检查
	interfaceResult := InterfaceProcessor("接口测试")
	fmt.Printf("接口结果: %v (类型: %T)\n", interfaceResult, interfaceResult)
	fmt.Println()

	// 7. 演示类型擦除后的实际情况
	fmt.Println("7. 类型擦除演示:")
	fmt.Println("编译后，泛型函数会为每种类型生成专门的版本:")
	fmt.Println("GenericProcessor[string] -> GenericProcessor_string")
	fmt.Println("GenericProcessor[int] -> GenericProcessor_int")
	fmt.Println("运行时不再有泛型参数，只有具体的类型操作")
	fmt.Println()

	// 8. 内存布局演示
	fmt.Println("8. 内存布局对比:")
	var stringContainer2 Container[string]
	var intContainer2 Container[int]
	fmt.Printf("Container[string] 大小: %d 字节\n", unsafe.Sizeof(stringContainer2))
	fmt.Printf("Container[int] 大小: %d 字节\n", unsafe.Sizeof(intContainer2))
	fmt.Println("每种类型组合都有确定的内存布局，无需额外的类型信息存储")
	fmt.Println()

	// 9. 编译时优化演示
	fmt.Println("9. 编译时优化的好处:")
	fmt.Println("✓ 零运行时开销 - 没有类型转换")
	fmt.Println("✓ 内联优化 - 编译器可以更好地优化")
	fmt.Println("✓ 缓存友好 - 确定的内存布局")
	fmt.Println("✓ 类型安全 - 编译时捕获类型错误")
}

// 编译时会生成类似这样的具体函数（概念性展示）:
/*
编译器实际生成的代码（简化版本）:

func GenericProcessor_string(value string, name string) string {
    fmt.Printf("处理 %s: 类型=string, 值=%v\n", name, value)
    return value
}

func GenericProcessor_int(value int, name string) int {
    fmt.Printf("处理 %s: 类型=int, 值=%v\n", name, value)
    return value
}

type Container_string struct {
    Value string
    Name  string
}

type Container_int struct {
    Value int
    Name  string
}

这就是"类型擦除"的本质：
1. 编译时：泛型参数被具体类型替换
2. 运行时：执行的是具体类型的代码，没有泛型开销
3. 类型安全：编译器确保所有类型匹配
*/