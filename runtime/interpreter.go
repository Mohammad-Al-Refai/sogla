package runtime

import (
	"fmt"
	"os"

	"m.shebli.refaai/ht/lexer"
)

type Parameters map[string]EvalValue

type Interpreter struct {
	Scope            *Scope
	AST              lexer.Program
	IsFinish         bool
	CurrentStatement lexer.Statement
	CurrentIndex     int
}

func (interpreter *Interpreter) next() {
	interpreter.CurrentIndex++
	if interpreter.CurrentIndex < len(interpreter.AST.Statements) {
		interpreter.CurrentStatement = interpreter.AST.Statements[interpreter.CurrentIndex]
	} else {
		interpreter.IsFinish = true
	}
}
func NewInterpreter(ast lexer.Program) *Interpreter {
	return &Interpreter{
		AST:              ast,
		Scope:            GlobalScope(),
		CurrentStatement: ast.Statements[0],
		CurrentIndex:     0,
	}
}
func (interpreter *Interpreter) threwError(message string) {
	fmt.Println(fmt.Errorf(fmt.Sprintf("[RuntimeError] %v", message)))
	os.Exit(1)
}
func (interpreter *Interpreter) Run() {
	for {
		interpreter.Evaluate(interpreter.CurrentStatement, interpreter.Scope)
		interpreter.next()
		if interpreter.IsFinish {
			return
		}
	}

}
func (interpreter *Interpreter) Evaluate(statement lexer.Statement, scope *Scope) EvalValue {
	switch statement.Kind {
	case lexer.K_OPEN_TAG:
		return interpreter.EvaluateOpenTag(statement.Body.(lexer.OpenTag), scope)
	case lexer.K_CLOSE_TAG:
		return interpreter.EvaluateCloseTag(statement.Body.(lexer.CloseTag), scope)
	case lexer.K_PARAMETER_VALUE:
		return interpreter.Evaluate(statement.Body.(lexer.Statement), scope)
	case lexer.K_IDENTIFIER:
		return interpreter.EvaluateIdentifier(statement.Body.(string), scope)
	case lexer.K_IF_STATEMENT:
		return interpreter.EvaluateIfStatement(statement.Body.(lexer.OpenTag), scope)
	case lexer.K_EXPRESSION:
		return interpreter.EvaluateExpression(statement.Body.(lexer.Expression), scope)
	case lexer.K_NUMBER:
		return EvalValue{Type: VAR_TYPE_NUMBER, Value: statement.Body.(int)}
	case lexer.K_STRING:
		return EvalValue{Type: VAR_TYPE_STRING, Value: statement.Body.(string)}
	case lexer.K_EOF:
		return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
	default:
		println(statement.Kind.String(), " unknown")
		return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
	}
}

func (interpreter *Interpreter) EvaluateOpenTag(openTag lexer.OpenTag, scope *Scope) EvalValue {
	children := openTag.Children
	newScope := Scope{}
	for _, child := range children {
		switch child.Kind {
		case lexer.K_IF_STATEMENT:
			return interpreter.EvaluateIfStatement(child.Body.(lexer.OpenTag), &newScope)
		case lexer.K_CLOSE_TAG:
			interpreter.EvaluateCloseTag(child.Body.(lexer.CloseTag), &newScope)
		case lexer.K_OPEN_TAG:
			interpreter.Evaluate(child, &newScope)
		}
	}
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}

func (interpreter *Interpreter) EvaluateCloseTag(closeTag lexer.CloseTag, scope *Scope) EvalValue {
	name := closeTag.Name
	isKeyword, _ := lexer.IsKeyword(name)
	isInScope, variable := interpreter.Scope.GetVariable(name)
	if isKeyword && name == "Let" {
		return interpreter.EvaluateLetDeclaration(closeTag, scope)
	}
	if isInScope && variable.ValueType == VAR_TYPE_NATIVE_FUNCTION {
		return interpreter.EvaluateNativeFunction(
			variable.Value.(RuntimeFunctionCall),
			interpreter.EvaluateParameters(closeTag.Params, scope))
	}
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}

func (interpreter *Interpreter) EvaluateLetDeclaration(closeTag lexer.CloseTag, scope *Scope) EvalValue {
	params := closeTag.Params
	evaluatedParams := interpreter.EvaluateParameters(params, scope)
	id, isId := evaluatedParams["id"]
	value, isValue := evaluatedParams["value"]

	if !isValue {
		interpreter.threwError("Expect 'value' param")
		return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
	}
	if !isId {
		interpreter.threwError("Expect 'id' param")
		return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
	}

	isOk := interpreter.Scope.DefineVariable(Variable{Name: id.Value.(string), Value: value.Value, ValueType: value.Type})
	if !isOk {
		interpreter.threwError(fmt.Sprintf("%v is already declared", id))
	}
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}

func (interpreter *Interpreter) EvaluateParameters(parameters []lexer.Parameter, scope *Scope) Parameters {
	params := make(Parameters)
	for _, param := range parameters {
		params[param.Key] = interpreter.Evaluate(param.Value, scope)
	}
	return params
}
func (interpreter *Interpreter) EvaluateIdentifier(name string, scope *Scope) EvalValue {
	isDefined, variable := interpreter.Scope.GetVariable(name)
	if isDefined {
		return EvalValue{Type: variable.ValueType, Value: variable.Value}
	}
	interpreter.threwError(fmt.Sprintf("'%v' is undefined", name))
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}

func (interpreter *Interpreter) EvaluateNativeIfStatement(function RuntimeFunctionCall, params Parameters) EvalValue {
	return function.Call(params)
}

func (interpreter *Interpreter) EvaluateNativeFunction(function RuntimeFunctionCall, params Parameters) EvalValue {
	return function.Call(params)
}
func (interpreter *Interpreter) EvaluateIfStatement(openTag lexer.OpenTag, scope *Scope) EvalValue {
	params := openTag.Params
	nodes := openTag.Children
	if len(params) == 0 {
		interpreter.threwError("Expect 'condition' param for if statement")
	}
	result := interpreter.EvaluateCondition(params[0], scope)
	if result.Value == true {
		for _, node := range nodes {
			interpreter.Evaluate(node, scope)
		}
	}
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}
func (interpreter *Interpreter) EvaluateCondition(param lexer.Parameter, scope *Scope) EvalValue {
	if param.Key != "condition" {
		interpreter.threwError(fmt.Sprintf("Expect 'condition' param for if statement found '%v'", param.Key))
	}
	return interpreter.Evaluate(param.Value, scope)
}

func (interpreter *Interpreter) EvaluateExpression(expr lexer.Expression, scope *Scope) EvalValue {
	for _, ex := range expr.Statements {
		if ex.Kind == lexer.K_OPERATOR {
			scope.Push(interpreter.EvaluateOperator(ex, scope))
			continue
		}
		scope.Push(interpreter.Evaluate(ex, scope))
	}
	return scope.Pop()
}

func (interpreter *Interpreter) EvaluateOperator(expr lexer.Statement, scope *Scope) EvalValue {
	fmt.Printf("OP %+v\n", expr)
	if expr.Body == "+" {
		return interpreter.Sum(scope.Pop(), scope.Pop())
	}
	if expr.Body == "*" {
		return interpreter.Mul(scope.Pop(), scope.Pop())
	}
	if expr.Body == "/" {
		return interpreter.Div(scope.Pop(), scope.Pop())
	}
	if expr.Body == "-" {
		return interpreter.Sub(scope.Pop(), scope.Pop())
	}
	if expr.Body == "greater" {
		return interpreter.GreaterThan(scope.Pop(), scope.Pop())
	}
	if expr.Body == "smaller" {
		return interpreter.SmallerThan(scope.Pop(), scope.Pop())
	}
	if expr.Body == "==" {
		return interpreter.Equal(scope.Pop(), scope.Pop())
	}
	if expr.Body == "!=" {
		return interpreter.NotEqual(scope.Pop(), scope.Pop())
	}
	return EvalValue{Type: VAR_TYPE_UNDEFINED, Value: "undefined"}
}

func (interpreter *Interpreter) Sum(x EvalValue, y EvalValue) EvalValue {
	fmt.Printf("Sum => %+v %+v\n", x, y)
	if !(x.IsNumber() || x.IsString() || y.IsNumber() || y.IsString()) {
		interpreter.threwError(fmt.Sprintf("expect string or number found %v and %v", y.Type.String(), x.Type.String()))
	}
	if x.IsString() && y.IsString() {
		return EvalValue{Type: VAR_TYPE_NUMBER, Value: y.Value.(string) + x.Value.(string)}
	}
	if x.IsNumber() && y.IsNumber() {
		return EvalValue{Type: VAR_TYPE_NUMBER, Value: y.Value.(int) + x.Value.(int)}
	}
	if x.IsNumber() || x.IsString() && y.IsNumber() || y.IsString() {
		interpreter.threwError(fmt.Sprintf("expect left and right to be number found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_NUMBER, Value: y.Value.(int) + x.Value.(int)}
}
func (interpreter *Interpreter) Mul(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || y.IsNumber()) {
		interpreter.threwError(fmt.Sprintf("expect both number found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_NUMBER, Value: x.Value.(int) * y.Value.(int)}
}
func (interpreter *Interpreter) Div(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || y.IsNumber()) {
		interpreter.threwError(fmt.Sprintf("expect both  number found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_NUMBER, Value: x.Value.(int) / y.Value.(int)}
}
func (interpreter *Interpreter) Sub(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || y.IsNumber()) {
		interpreter.threwError(fmt.Sprintf("expect both number found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_NUMBER, Value: x.Value.(int) - y.Value.(int)}
}
func (interpreter *Interpreter) GreaterThan(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || y.IsNumber()) {
		interpreter.threwError(fmt.Sprintf("expect both number or boolean found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_BOOLEAN, Value: y.Value.(int) > x.Value.(int)}
}
func (interpreter *Interpreter) SmallerThan(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || y.IsNumber()) {
		interpreter.threwError(fmt.Sprintf("expect both number or boolean found %v and %v", y.Type.String(), x.Type.String()))
	}
	return EvalValue{Type: VAR_TYPE_BOOLEAN, Value: y.Value.(int) < x.Value.(int)}
}
func (interpreter *Interpreter) Equal(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || x.IsString() || y.IsNumber() || y.IsString()) {
		interpreter.threwError(fmt.Sprintf("expect string or number found %v and %v", y.Type.String(), x.Type.String()))
	}
	if x.IsString() && y.IsString() {
		return EvalValue{Type: VAR_TYPE_NUMBER, Value: y.Value.(string) == x.Value.(string)}
	}
	return EvalValue{Type: VAR_TYPE_BOOLEAN, Value: y.Value.(int) == x.Value.(int)}
}
func (interpreter *Interpreter) NotEqual(x EvalValue, y EvalValue) EvalValue {
	if !(x.IsNumber() || x.IsString() || y.IsNumber() || y.IsString()) {
		interpreter.threwError(fmt.Sprintf("expect string or number found %v and %v", y.Type.String(), x.Type.String()))
	}
	if x.IsString() && y.IsString() {
		return EvalValue{Type: VAR_TYPE_NUMBER, Value: y.Value.(string) != x.Value.(string)}
	}
	return EvalValue{Type: VAR_TYPE_BOOLEAN, Value: y.Value.(int) != x.Value.(int)}
}
