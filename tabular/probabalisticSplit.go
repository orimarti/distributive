package tabular

import (
	log "github.com/Sirupsen/logrus"
	"math"
	"regexp"
)

// This is a lot of helper functions. To keep things straight, here's a diagram
// of where they're used.
// mean -> sqrDiff -> variance -> stdDev -> normal -> chauvenet -> ProbabalisticSplit
// [fork]          -> chauvenet -> ProbabalisticSplit

// meanInt uses sum to find the mean of a list of ints.
func meanInt(data []int) float64 {
	// sumInt finds the sum of a list of ints. Exciting, I know.
	sumInt := func(data []int) (sum int) {
		for _, i := range data {
			sum += i
		}
		return sum
	}
	return float64(sumInt(data)) / float64(len(data))
}

// meanFloat uses sum to find the mean of a list of float.
func meanFloat(data []float64) float64 {
	// sumFloat finds the sum of a list of floats. Exciting, I know.
	sumFloat := func(data []float64) (sum float64) {
		for _, i := range data {
			sum += i
		}
		return sum
	}
	return sumFloat(data) / float64(len(data))
}

// getSquaredDifferences returns a list of the squared differences between each
// point and the mean. Used in variance and chauvenet->highestVarianceIndex
func getSquaredDifferences(data []int) (sqrDiffs []float64) {
	xbar := meanInt(data)
	for _, xn := range data {
		sqrDiff := math.Pow((xbar - float64(xn)), 2)
		sqrDiffs = append(sqrDiffs, sqrDiff)
	}
	return sqrDiffs
}

// variance is the mean of the squared differences between each observed
// point and the mean (xbar)
func variance(data []int) float64 {
	return meanFloat(getSquaredDifferences(data))
}

// stdDev returns the standard deviation of a data set of integers
func stdDev(data []int) float64 {
	return math.Sqrt(variance(data))
}

// normalDistribution returns the probability that the data point x
// lands where it does, based on the mean (mu) and standard deviation (sigma)
func normalDistribution(x float64, mu float64, sigma float64) float64 {
	coefficient := 1 / (sigma * math.Sqrt(2*math.Pi))
	exponent := -(math.Pow(x-mu, 2) / (2 * math.Pow(sigma, 2)))
	return coefficient * math.Pow(math.E, exponent)
}

// compare is a function that compares to floats. It is used for sorting and
// finding - see extremaIndex
type compare func(x float64, y float64) bool

// maxFunc is an instance of compare that can be used to find the greater
// of two values
var maxFunc compare = func(x float64, y float64) bool { return x > y }

// minFunc is like maxFunc, but wiht the lesser of the two values
var minFunc compare = func(x float64, y float64) bool { return x < y }

// extremaIndex finds the index of a value in a list that when compared with
// the any of other data with comparisonFunc will return true. It is most easily
// applicable in finding maxes and mins
func extremaIndex(comparisonFunc compare, data []float64) (index int) {
	if len(data) < 1 {
		return 0
	}
	extrema := data[0]
	extremaIndex := 0
	for i, datum := range data {
		if comparisonFunc(datum, extrema) {
			extrema = datum
			extremaIndex = i
		}
	}
	return extremaIndex
}

// chauvenet simply takes a slice of integers, applies Chauvenet's Criterion,
// and potentially discards a single outlier. Not necessarily, though!
// https://en.wikipedia.org/wiki/Chauvenet%27s_criterion
func chauvenet(data []int) (result []int) {
	// isOutlier applies chauvenet's criterion to determine whether or not
	// x is an outlier
	isOutlier := func(x float64, data []int) bool {
		xbar := meanInt(data)
		sigma := stdDev(data)
		probability := normalDistribution(x, xbar, sigma)
		if probability < float64(1)/float64((2*len(data))) {
			return true
		}
		return false
	}
	// if its an outlier, cut it out. If not, leave it in.
	// find the index with the highest variance
	index := extremaIndex(maxFunc, getSquaredDifferences(data))
	potentialOutlier := float64(data[index])
	// test if that datum is an outlier
	if isOutlier(potentialOutlier, data) {
		return append(data[:index], data[index+1:]...)
	}
	return data
}

// getColumnRegex is the core of the logic. It determines which regex most
// accurately splits the data into columns by testing the deviation in the
// row lengths using different regexps.
func getColumnRegex(str string, rowSep *regexp.Regexp) *regexp.Regexp {
	// matchesMost is used to ensure that our regexp actually is splitting the
	// lines of a table, instead of just returning them whole.
	matchesMost := func(re *regexp.Regexp, rows []string) bool {
		count := 0
		for _, row := range rows {
			if re.MatchString(row) {
				count++
			}
		}
		return count >= (len(rows) / 2)
	}
	// getRowLengths returns row length counts for each table
	getRowLengths := func(table Table) (lengths []int) {
		for _, row := range table {
			lengths = append(lengths, len(row))
		}
		return lengths
	}
	// getVariance returns the variance of the split provided by a regexp,
	// after discarding a number of outliers
	getVariance := func(colSep *regexp.Regexp, outliers int) float64 {
		table := SeparateString(rowSep, colSep, str)
		rowLengths := getRowLengths(table)
		for i := 0; i < outliers; i++ {
			rowLengths = chauvenet(rowLengths)
		}
		return variance(rowLengths)
	}
	// testRegexp determines whether or not a given regexp gives perfectly even
	// line lengths, including discarding of a number of outliers
	testRegexp := func(colSep *regexp.Regexp, outliers int) bool {
		for i := 0; i < outliers; i++ {
			variance := getVariance(colSep, i)
			if variance <= .1 {
				return true
			}
		}
		return false
	}
	// different column separators to try out
	initialColSeps := []*regexp.Regexp{
		regexp.MustCompile(`\t+`),    // tabs
		regexp.MustCompile(`\s{4}`),  // exactly four whitespaces
		regexp.MustCompile(`\s{2,}`), // two+ whitespace (spaces in cols)
		regexp.MustCompile(`\s+`),    // any whitespace
	}
	// filter regexps that have no matches at all - they will always return
	// rows of even length (length 1).
	colSeps := []*regexp.Regexp{}
	rows := rowSep.Split(str, -1)
	for _, re := range initialColSeps {
		if matchesMost(re, rows) {
			colSeps = append(colSeps, re)
		}
	}
	if len(colSeps) < 1 {
		log.WithFields(log.Fields{
			"attempted": initialColSeps,
			"table":     str,
		}).Warn("ProbabalisticSplit couldn't find a column separator.")
		colSeps = initialColSeps
	}
	// discarding up to passes outliers, test each regexp for row length
	// consistency
	passes := 3
	for i := 0; i < passes; i++ {
		for _, re := range colSeps {
			if testRegexp(re, i) {
				return re
			}
		}
	}
	// if still not done, just pick the one with the lowest variance
	log.WithFields(log.Fields{
		"attempted": initialColSeps,
		"outliers":  passes,
	}).Debug("ProbabalisticSplit couldn't find a consistent regexp")
	var variances []float64
	for _, colSep := range colSeps {
		variances = append(variances, getVariance(colSep, passes))
	}
	// ensure that index can be found in tables
	minVarianceIndex := extremaIndex(minFunc, variances)
	if len(colSeps) <= minVarianceIndex {
		msg := "Internal error: minVarianceIndex couldn't be found in colSeps"
		log.WithFields(log.Fields{
			"index":   minVarianceIndex,
			"colSeps": colSeps,
		}).Fatal(msg)
	}
	return colSeps[minVarianceIndex]
}

// ProbabalisticSplit splits a string based on the regexp that gives the most
// consistent line length (potentially discarding one outlier line length).
func ProbabalisticSplit(str string) (output Table) {
	colSep := getColumnRegex(str, rowSep)
	log.WithFields(log.Fields{
		"regexp": colSep.String(),
	}).Debug("ProbabalisticSplit chose a regexp")
	return SeparateString(rowSep, colSep, str)
}
