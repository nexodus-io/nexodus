// Runes a SQL statement against the DB
//
//	When I run SQL "UPDATE connectors SET connector_type_id='foo' WHERE id = '${connector_id}';" expect 1 row to be affected.
//
// Runes a SQL statement against the DB and check the results
//
//	And I run SQL "SELECT count(*) from connector_deployments where connector_id='${connector_id}'" gives results:
//	  | count |
//	  | 0     |
package cucumber

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"github.com/olekukonko/tablewriter"
	"github.com/pmezard/go-difflib/difflib"
)

func init() {
	StepModules = append(StepModules, func(ctx *godog.ScenarioContext, s *TestScenario) {
		ctx.Step(`^I run SQL "([^"]*)" expect (\d+) row to be affected\.$`, s.iRunSQLExpectRowToBeAffected)
		ctx.Step(`^I run SQL "([^"]*)" gives results:$`, s.iRunSQLGivesResults)
	})
}

func (s *TestScenario) iRunSQLExpectRowToBeAffected(sql string, expected int64) error {

	var err error
	sql, err = s.Expand(sql, []string{})
	if err != nil {
		return err
	}

	gorm := s.DB
	exec := gorm.Exec(sql)
	if exec.Error != nil {
		return exec.Error
	}
	if exec.RowsAffected != expected {
		return fmt.Errorf("expected %d rows to be affected but %d were affected", expected, exec.RowsAffected)
	}
	return nil
}

//type TableRow = messages.PickleStepArgument_PickleTable_PickleTableRow
//type TableCell = messages.PickleStepArgument_PickleTable_PickleTableRow_PickleTableCell

func (s *TestScenario) iRunSQLGivesResults(sql string, expected *godog.Table) error {

	var err error
	sql, err = s.Expand(sql, []string{})
	if err != nil {
		return err
	}

	gorm := s.DB

	rows, err := gorm.Raw(sql).Rows()
	if err != nil {
		return err
	}
	defer util.IgnoreError(rows.Close())

	var actualTable [][]string
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	actualTable = append(actualTable, cols)

	// add the header rows.
	for rows.Next() {

		columns := make([]interface{}, len(cols))
		columnsPtr := make([]interface{}, len(cols))
		for i := range columns {
			columnsPtr[i] = &columns[i]
		}

		err = rows.Scan(columnsPtr...)
		if err != nil {
			return err
		}

		var rowString []string
		for _, c := range columns {
			cell := fmt.Sprintf("%v", c)
			rowString = append(rowString, cell)
		}
		actualTable = append(actualTable, rowString)
	}

	expectedTable := GodogTableToStringTable(expected)

	if !reflect.DeepEqual(expectedTable, actualTable) {
		expected := StringTableToCucumberTable(expectedTable)
		actual := StringTableToCucumberTable(actualTable)

		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expected),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}
	return nil
}

func GodogTableToStringTable(table *godog.Table) [][]string {
	data := make([][]string, len(table.Rows))
	for r, row := range table.Rows {
		data[r] = make([]string, len(row.Cells))
		for c, cell := range row.Cells {
			data[r][c] = cell.Value
		}
	}
	return data
}

func StringTableToCucumberTable(data [][]string) string {
	buf := &strings.Builder{}
	table := tablewriter.NewWriter(buf)
	table.SetBorders(tablewriter.Border{
		Left:   true,
		Right:  true,
		Top:    false,
		Bottom: false,
	})
	table.AppendBulk(data)
	table.Render()
	return buf.String()
}
