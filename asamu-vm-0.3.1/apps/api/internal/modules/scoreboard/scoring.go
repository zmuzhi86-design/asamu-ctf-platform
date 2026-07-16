package scoreboard

import "math"

type DynamicRule struct{ Maximum, Minimum, Decay int }

func DynamicScore(rule DynamicRule, solves int64) int {
	if rule.Maximum <= 0 {
		return 0
	}
	if rule.Minimum < 0 {
		rule.Minimum = 0
	}
	if rule.Minimum > rule.Maximum {
		rule.Minimum = rule.Maximum
	}
	if rule.Decay <= 0 {
		return rule.Maximum
	}
	solve := float64(solves)
	decay := float64(rule.Decay)
	value := float64(rule.Minimum) + float64(rule.Maximum-rule.Minimum)*(decay*decay)/(decay*decay+solve*solve)
	score := int(math.Round(value))
	if score < rule.Minimum {
		return rule.Minimum
	}
	if score > rule.Maximum {
		return rule.Maximum
	}
	return score
}
func BloodBonus(rank int, base int) int {
	switch rank {
	case 1:
		return max(50, base/10)
	case 2:
		return max(30, base/20)
	case 3:
		return max(10, base/40)
	default:
		return 0
	}
}
