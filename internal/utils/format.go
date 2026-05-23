package utils

import (
	"fmt"
	"strconv"
)

func FormatMoney(amount float64) string {
	if amount < 0 {
		return fmt.Sprintf("-$%.2f", -amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

func SFormatMoney(amount string) string {
	valu, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return amount
	}
	if valu < 0 {
		return fmt.Sprintf("-$%.2f", -valu)
	}
	return fmt.Sprintf("$%.2f", valu)
}
