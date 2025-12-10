package utils

import (
	"testing"
)

func TestMoneyCreation(t *testing.T) {
	t.Run("NewMoney", func(t *testing.T) {
		m := NewMoney(10, 50)
		if m.ToCents() != 1050 {
			t.Errorf("Expected 1050 cents, got %d", m.ToCents())
		}
	})

	t.Run("Cents", func(t *testing.T) {
		m := Cents(1234)
		if m.ToCents() != 1234 {
			t.Errorf("Expected 1234 cents, got %d", m.ToCents())
		}
	})

	t.Run("Dollars", func(t *testing.T) {
		m := Dollars(100)
		if m.ToCents() != 10000 {
			t.Errorf("Expected 10000 cents, got %d", m.ToCents())
		}
	})

	t.Run("FromFloat", func(t *testing.T) {
		m := FromFloat(19.99)
		if m.ToCents() != 1999 {
			t.Errorf("Expected 1999 cents, got %d", m.ToCents())
		}

		m = FromFloat(-5.75)
		if m.ToCents() != -575 {
			t.Errorf("Expected -575 cents, got %d", m.ToCents())
		}
	})
}

func TestMoneyParts(t *testing.T) {
	m := NewMoney(123, 45)

	if m.DollarsPart() != 123 {
		t.Errorf("Expected 123 dollars, got %d", m.DollarsPart())
	}

	if m.CentsPart() != 45 {
		t.Errorf("Expected 45 cents, got %d", m.CentsPart())
	}
}

func TestMoneyArithmetic(t *testing.T) {
	m1 := NewMoney(10, 50)
	m2 := NewMoney(5, 25)

	t.Run("Add", func(t *testing.T) {
		result := m1.Add(m2)
		if result.ToCents() != 1575 {
			t.Errorf("Expected 1575 cents, got %d", result.ToCents())
		}
	})

	t.Run("Sub", func(t *testing.T) {
		result := m1.Sub(m2)
		if result.ToCents() != 525 {
			t.Errorf("Expected 525 cents, got %d", result.ToCents())
		}
	})

	t.Run("Mul", func(t *testing.T) {
		result := m2.Mul(3)
		if result.ToCents() != 1575 {
			t.Errorf("Expected 1575 cents, got %d", result.ToCents())
		}
	})

	t.Run("Div", func(t *testing.T) {
		m := NewMoney(100, 0)
		result := m.Div(4)
		if result.ToCents() != 2500 {
			t.Errorf("Expected 2500 cents, got %d", result.ToCents())
		}
	})

	t.Run("MulFloat", func(t *testing.T) {
		m := NewMoney(100, 0)
		result := m.MulFloat(0.15) // 15%
		if result.ToCents() != 1500 {
			t.Errorf("Expected 1500 cents, got %d", result.ToCents())
		}
	})

	t.Run("Percentage", func(t *testing.T) {
		m := NewMoney(200, 0)
		result := m.Percentage(10) // 10%
		if result.ToCents() != 2000 {
			t.Errorf("Expected 2000 cents, got %d", result.ToCents())
		}
	})
}

func TestMoneyComparison(t *testing.T) {
	m1 := NewMoney(10, 0)
	m2 := NewMoney(20, 0)
	m3 := NewMoney(10, 0)

	t.Run("Cmp", func(t *testing.T) {
		if m1.Cmp(m2) != -1 {
			t.Error("Expected m1 < m2")
		}
		if m2.Cmp(m1) != 1 {
			t.Error("Expected m2 > m1")
		}
		if m1.Cmp(m3) != 0 {
			t.Error("Expected m1 == m3")
		}
	})

	t.Run("Min", func(t *testing.T) {
		result := m1.Min(m2)
		if result != m1 {
			t.Error("Expected min to be m1")
		}
	})

	t.Run("Max", func(t *testing.T) {
		result := m1.Max(m2)
		if result != m2 {
			t.Error("Expected max to be m2")
		}
	})
}

func TestMoneyAbs(t *testing.T) {
	m := NewMoney(-50, 0)
	result := m.Abs()
	if result.ToCents() != 5000 {
		t.Errorf("Expected 5000 cents, got %d", result.ToCents())
	}
}

func TestMoneyString(t *testing.T) {
	m := NewMoney(1234, 56)
	str := m.String()
	if str != "1234.56" {
		t.Errorf("Expected '1234.56', got '%s'", str)
	}

	// Negative money: use Cents directly for negative values
	m = Cents(-5075)
	str = m.String()
	if str != "-50.75" {
		t.Errorf("Expected '-50.75', got '%s'", str)
	}
}

func TestMoneyFormat(t *testing.T) {
	m := NewMoney(1234567, 89)

	t.Run("USD", func(t *testing.T) {
		str := m.Format("USD")
		if str != "$1,234,567.89" {
			t.Errorf("Expected '$1,234,567.89', got '%s'", str)
		}
	})

	t.Run("EUR", func(t *testing.T) {
		str := m.Format("EUR")
		// EUR uses . as thousands sep and , as decimal sep
		if str != "€1.234.567,89" {
			t.Errorf("Expected '€1.234.567,89', got '%s'", str)
		}
	})

	t.Run("GBP", func(t *testing.T) {
		str := m.Format("GBP")
		if str != "£1,234,567.89" {
			t.Errorf("Expected '£1,234,567.89', got '%s'", str)
		}
	})

	t.Run("JPY - no decimals", func(t *testing.T) {
		// JPY has no decimal places, so the amount is in yen directly
		yen := Cents(123456) // This represents 123456 yen (since JPY has 0 decimal places, we treat cents as yen)
		str := yen.Format("JPY")
		if str != "¥123,456" {
			t.Errorf("Expected '¥123,456', got '%s'", str)
		}
	})
}

func TestMoneySplit(t *testing.T) {
	m := NewMoney(100, 0) // $100

	t.Run("Even split", func(t *testing.T) {
		parts := m.Split(4)
		if len(parts) != 4 {
			t.Errorf("Expected 4 parts, got %d", len(parts))
		}

		total := Cents(0)
		for _, p := range parts {
			total = total.Add(p)
		}
		if total != m {
			t.Errorf("Split parts don't sum to original: %d != %d", total.ToCents(), m.ToCents())
		}

		// Each part should be $25
		for _, p := range parts {
			if p.ToCents() != 2500 {
				t.Errorf("Expected each part to be 2500 cents, got %d", p.ToCents())
			}
		}
	})

	t.Run("Uneven split", func(t *testing.T) {
		m := Cents(1000) // $10.00
		parts := m.Split(3)

		total := Cents(0)
		for _, p := range parts {
			total = total.Add(p)
		}
		if total != m {
			t.Errorf("Split parts don't sum to original: %d != %d", total.ToCents(), m.ToCents())
		}

		// First part should get the extra cent
		if parts[0].ToCents() != 334 {
			t.Errorf("Expected first part to be 334 cents, got %d", parts[0].ToCents())
		}
		if parts[1].ToCents() != 333 {
			t.Errorf("Expected second part to be 333 cents, got %d", parts[1].ToCents())
		}
		if parts[2].ToCents() != 333 {
			t.Errorf("Expected third part to be 333 cents, got %d", parts[2].ToCents())
		}
	})
}

func TestRandomAmount(t *testing.T) {
	rng := NewRandom(42)

	min := Dollars(10)
	max := Dollars(100)

	for i := 0; i < 1000; i++ {
		m := RandomAmount(rng, min, max)
		if m < min || m > max {
			t.Errorf("RandomAmount returned %d, expected between %d and %d", m.ToCents(), min.ToCents(), max.ToCents())
		}
	}
}

func TestMoneyRoundToNearest(t *testing.T) {
	t.Run("Round to $5", func(t *testing.T) {
		m := NewMoney(123, 0)
		result := m.RoundToNearest(Dollars(5))
		if result.ToCents() != 12500 {
			t.Errorf("Expected 12500 cents ($125), got %d", result.ToCents())
		}
	})

	t.Run("Round to $10", func(t *testing.T) {
		m := NewMoney(127, 0)
		result := m.RoundToNearest(Dollars(10))
		if result.ToCents() != 13000 {
			t.Errorf("Expected 13000 cents ($130), got %d", result.ToCents())
		}
	})
}
