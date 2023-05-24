package model

import (
	"errors"
	"math"
)

type Matrix struct {
	// Number of Rows
	Rows int `json:"rows"`
	// Number of columns
	Cols int `json:"cols"`
	// Matrix stored as a flat array: Aij = Elements[i*step + j]
	Elements []float64 `json:"elements"`
	// Offset between Rows
	step int
}

const MAX_ONE_ARRAY = 100

var oneArray = make([]float64, MAX_ONE_ARRAY)

func init() {
	for i := 0; i < MAX_ONE_ARRAY; i += 1 {
		oneArray[i] = 1
	}
}

func NewMatrix(rows, cols int) *Matrix {
	return MakeMatrix(make([]float64, rows*cols), rows, cols)
}

func MakeMatrix(Elements []float64, rows, cols int) *Matrix {
	A := new(Matrix)
	A.Rows = rows
	A.Cols = cols
	A.step = cols
	A.Elements = Elements
	return A
}

func NewUniformMatrix(rows, cols int) *Matrix {
	return MakeMatrix(oneArray[:rows*cols], rows, cols)
}

func NewCopiedMatrixFromElements(Elements []float64, rows, cols int) *Matrix {
	A := NewMatrix(rows, cols)
	copy(A.Elements, Elements)

	return A
}

func (A *Matrix) CountRows() int {
	return A.Rows
}

func (A *Matrix) CountCols() int {
	return A.Cols
}

func (A *Matrix) GetElm(row int, col int) float64 {
	return A.Elements[row*A.step+col]
}

func (A *Matrix) SetElm(row int, col int, v float64) {
	A.Elements[row*A.step+col] = v
}

func (A *Matrix) RowSum() *Matrix {
	B := NewUniformMatrix(A.Cols, 1)
	rRows := int(math.Max(float64(A.Rows), float64(B.Rows)))
	rCols := int(math.Max(float64(A.Cols), float64(B.Cols)))

	result := MakeMatrix(make([]float64, rCols*rRows), rRows, rCols)

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < B.Cols; j++ {
			sum := float64(0)
			for k := 0; k < A.Cols; k++ {
				sum += A.GetElm(i, k) * B.GetElm(k, j)
			}
			result.SetElm(i, j, sum)
		}
	}

	return result
}

func (A *Matrix) diagonalCopy() []float64 {
	diag := make([]float64, A.Cols)
	for i := 0; i < len(diag); i++ {
		diag[i] = A.GetElm(i, i)
	}
	return diag
}

func (A *Matrix) copy() *Matrix {
	B := new(Matrix)
	B.Rows = A.Rows
	B.Cols = A.Cols
	B.step = A.step

	B.Elements = make([]float64, A.Cols*A.Rows)

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			B.Elements[i*A.step+j] = A.GetElm(i, j)
		}
	}
	return B
}

func (A *Matrix) trace() float64 {
	var tr float64 = 0
	for i := 0; i < A.Cols; i++ {
		tr += A.GetElm(i, i)
	}
	return tr
}

func (A *Matrix) Add(B *Matrix) error {
	/*  Just for convenience!
	If base matrix is not defined yet create a element array with zero value
	*/
	if A.Elements == nil && A.Rows == 0 && A.Cols == 0 {
		A.Elements = make([]float64, B.Cols*B.Rows)
		A.Rows = B.Rows
		A.Cols = B.Cols
		A.step = B.step
	}

	if A.Cols != B.Cols || A.Rows != B.Rows {
		if A.step != B.step {
			return errors.New("Wrong input sizes")
		}

		maxRow := int(math.Max(float64(A.Rows), float64(B.Rows)))
		maxCol := int(math.Max(float64(A.Cols), float64(B.Cols)))

		targetB := make([]float64, maxRow*maxCol)
		copy(targetB, B.Elements)
		B.Elements = targetB

		targetA := make([]float64, maxRow*maxCol)
		copy(targetA, A.Elements)
		A.Elements = targetA
		B.Cols, B.Rows = maxCol, maxRow
		A.Cols, A.Rows = maxCol, maxRow
	}

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			A.SetElm(i, j, A.GetElm(i, j)+B.GetElm(i, j))
		}
	}

	return nil
}

func (A *Matrix) substract(B *Matrix) error {
	if A.Cols != B.Cols && A.Rows != B.Rows {
		return errors.New("Wrong input sizes")
	}
	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			A.SetElm(i, j, A.GetElm(i, j)-B.GetElm(i, j))
		}
	}

	return nil
}

func (A *Matrix) Scale(a float64) {
	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			A.SetElm(i, j, a*A.GetElm(i, j))
		}
	}
}

func (A *Matrix) RoundToFixed(precision uint) *Matrix {
	ratio := math.Pow(10, float64(precision))
	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			A.SetElm(i, j, math.Round(A.GetElm(i, j)*ratio)/ratio)
		}
	}
	return A
}

func Add(A *Matrix, B *Matrix) *Matrix {
	result := MakeMatrix(make([]float64, A.Cols*A.Rows), A.Rows, A.Cols)

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			result.SetElm(i, j, A.GetElm(i, j)+B.GetElm(i, j))
		}
	}

	return result
}

func Substract(A *Matrix, B *Matrix) *Matrix {
	result := MakeMatrix(make([]float64, A.Cols*A.Rows), A.Cols, A.Rows)

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < A.Cols; j++ {
			result.SetElm(i, j, A.GetElm(i, j)-B.GetElm(i, j))
		}
	}

	return result
}

func Multiply(A *Matrix, B *Matrix) *Matrix {

	//rRows := int(math.Max(float64(A.Rows), float64(B.Rows)))
	//rCols := int(math.Max(float64(A.Cols), float64(B.Cols)))
	rRows := A.Rows
	rCols := B.Cols

	result := MakeMatrix(make([]float64, rCols*rRows), rRows, rCols)

	for i := 0; i < A.Rows; i++ {
		for j := 0; j < B.Cols; j++ {
			sum := float64(0)
			for k := 0; k < A.Cols; k++ {
				sum += A.GetElm(i, k) * B.GetElm(k, j)
			}
			result.SetElm(i, j, sum)
		}
	}

	return result
}

func Merge(A *Matrix, B *Matrix) *Matrix {
	if A.CountRows() != B.CountRows() {
		return A
	}

	elements := []float64{}
	for i := 0; i < A.CountRows(); i++ {
		n := i * A.CountCols()
		m := i * B.CountCols()
		elements = append(elements, A.Elements[n:n+A.CountCols()]...)
		elements = append(elements, B.Elements[m:m+B.CountCols()]...)
	}

	return MakeMatrix(elements, A.CountRows(), A.CountCols()+B.CountCols())
}

type QuotaMatrix struct {
	cpCatalog   map[string]int
	prodCatalog map[string]int
	quota       *Matrix
}

func NewQuotaMatrix(cpNames, prodNames []string) *QuotaMatrix {
	qm := &QuotaMatrix{
		cpCatalog:   make(map[string]int, len(cpNames)),
		prodCatalog: make(map[string]int, len(prodNames)),
	}
	for i, n := range cpNames {
		qm.cpCatalog[n] = i
	}
	for i, n := range prodNames {
		qm.prodCatalog[n] = i
	}
	lenC := len(prodNames)
	lenR := len(cpNames)
	qm.quota = MakeMatrix(make([]float64, lenC*lenR), lenR, lenC)
	return qm
}

func (q *QuotaMatrix) Add(cpName, prodName string, v float64) {
	cpIdx, cpok := q.cpCatalog[cpName]
	prodIdx, prodok := q.prodCatalog[prodName]

	if cpok && prodok {
		q.quota.SetElm(cpIdx, prodIdx, v)
	}
}

func (q *QuotaMatrix) GetQuota(counterPointName, prodName string) float64 {
	cpIdx := q.cpCatalog[counterPointName]
	prodIdx := q.prodCatalog[prodName]

	return q.quota.GetElm(cpIdx, prodIdx)
}

func (q *QuotaMatrix) GetAllocQuota(counterPointName, prodName string) float64 {
	cpIdx := q.cpCatalog[counterPointName]
	var v float64 = q.GetQuota(counterPointName, prodName)
	var s float64 = 0.0
	for i := 0; i < q.quota.Cols; i++ {
		s = s + q.quota.GetElm(cpIdx, i)
	}
	return 1 / (s * 100) * (v * 100)
}

func (q *QuotaMatrix) Validate() *Matrix {
	onesSlice := make([]float64, q.quota.Cols)
	for i := 0; i < len(onesSlice); i++ {
		onesSlice[i] = 1
	}
	tm := MakeMatrix(onesSlice, 3, 1)

	return Multiply(q.quota, tm)
}
