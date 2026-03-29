// Glicko-2 Rating System Implementation for acb-worker
// Based on: http://www.glicko.net/glicko/glicko2.pdf
package main

import "math"

const (
	glicko2Scale     = 173.7178
	glicko2Tau       = 0.5
	glicko2DefaultMu = 1500.0
	glicko2DefaultRD = 350.0
	glicko2Epsilon   = 1e-6
)

// Glicko2Rating represents a Glicko-2 rating.
type Glicko2Rating struct {
	Mu    float64 `json:"mu"`
	Phi   float64 `json:"phi"`
	Sigma float64 `json:"sigma"`
}

// DisplayRating returns the conservative rating estimate (mu - 2*phi).
func (r Glicko2Rating) DisplayRating() float64 {
	return r.Mu - 2*r.Phi
}

// toScale converts from Glicko-1 (mu,phi) to Glicko-2 internal scale.
func toScale(mu, phi float64) (float64, float64) {
	return (mu - glicko2DefaultMu) / glicko2Scale, phi / glicko2Scale
}

// fromScale converts from Glicko-2 internal scale back to Glicko-1.
func fromScale(mu2, phi2 float64) (float64, float64) {
	return mu2*glicko2Scale + glicko2DefaultMu, phi2 * glicko2Scale
}

// gFunc computes g(phi) = 1 / sqrt(1 + 3*phi^2/pi^2)
func gFunc(phi float64) float64 {
	return 1.0 / math.Sqrt(1.0+3.0*phi*phi/(math.Pi*math.Pi))
}

// eFunc computes E(mu, mu_j, phi_j)
func eFunc(mu, muJ, phiJ float64) float64 {
	return 1.0 / (1.0 + math.Exp(-gFunc(phiJ)*(mu-muJ)))
}

// computeVolatility implements Step 5 of Glicko-2 (Illinois algorithm).
func computeVolatility(sigma, phi, v, delta float64) float64 {
	a := math.Log(sigma * sigma)
	tau := glicko2Tau

	f := func(x float64) float64 {
		expX := math.Exp(x)
		tmp := phi*phi + v + expX
		return (expX*(delta*delta-phi*phi-v-expX))/(2*tmp*tmp) - (x-a)/(tau*tau)
	}

	A := a
	var B float64
	if delta*delta > phi*phi+v {
		B = math.Log(delta*delta - phi*phi - v)
	} else {
		k := 1.0
		for f(a-k*tau) < 0 {
			k++
		}
		B = a - k*tau
	}

	fA := f(A)
	fB := f(B)

	for math.Abs(B-A) > glicko2Epsilon {
		C := A + (A-B)*fA/(fB-fA)
		fC := f(C)
		if fC*fB <= 0 {
			A = B
			fA = fB
		} else {
			fA = fA / 2
		}
		B = C
		fB = fC
	}

	return math.Exp(A / 2)
}

type opponent struct {
	mu2   float64 // Glicko-2 scale
	phi2  float64 // Glicko-2 scale
	score float64 // 1.0 = win, 0.5 = draw, 0.0 = loss
}

// updateSingleRating updates a single player's rating given their opponents.
func updateSingleRating(r Glicko2Rating, opps []opponent) Glicko2Rating {
	if len(opps) == 0 {
		// No opponents: increase RD (rating decay)
		phi2 := r.Phi / glicko2Scale
		newPhi2 := math.Min(math.Sqrt(phi2*phi2+r.Sigma*r.Sigma), glicko2DefaultRD/glicko2Scale)
		return Glicko2Rating{
			Mu:    r.Mu,
			Phi:   newPhi2 * glicko2Scale,
			Sigma: r.Sigma,
		}
	}

	mu2, phi2 := toScale(r.Mu, r.Phi)

	// Step 3: Compute v (variance)
	vInverse := 0.0
	for _, o := range opps {
		gPhi := gFunc(o.phi2)
		e := eFunc(mu2, o.mu2, o.phi2)
		vInverse += gPhi * gPhi * e * (1 - e)
	}
	v := 1.0 / vInverse

	// Step 4: Compute delta
	deltaSum := 0.0
	for _, o := range opps {
		gPhi := gFunc(o.phi2)
		e := eFunc(mu2, o.mu2, o.phi2)
		deltaSum += gPhi * (o.score - e)
	}
	delta := v * deltaSum

	// Step 5: Compute new volatility
	newSigma := computeVolatility(r.Sigma, phi2, v, delta)

	// Step 6: Update phi*
	phiStar := math.Sqrt(phi2*phi2 + newSigma*newSigma)

	// Step 7: Update phi and mu
	newPhi2 := 1.0 / math.Sqrt(1.0/(phiStar*phiStar)+1.0/v)
	newMu2 := mu2 + newPhi2*newPhi2*deltaSum

	newMu, newPhi := fromScale(newMu2, newPhi2)

	return Glicko2Rating{
		Mu:    newMu,
		Phi:   newPhi,
		Sigma: newSigma,
	}
}

// UpdateRatings computes new ratings for all participants in a multi-player match.
// Scores are used pairwise: for each pair (i, j), player i gets:
//   - 1.0 if scores[i] > scores[j]
//   - 0.5 if scores[i] == scores[j]
//   - 0.0 if scores[i] < scores[j]
func UpdateRatings(ratings []Glicko2Rating, scores []float64) []Glicko2Rating {
	n := len(ratings)
	if n < 2 {
		return ratings
	}

	result := make([]Glicko2Rating, n)

	for i := 0; i < n; i++ {
		var opps []opponent
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			var s float64
			switch {
			case scores[i] > scores[j]:
				s = 1.0
			case scores[i] == scores[j]:
				s = 0.5
			default:
				s = 0.0
			}

			mu2, phi2 := toScale(ratings[j].Mu, ratings[j].Phi)
			opps = append(opps, opponent{mu2: mu2, phi2: phi2, score: s})
		}

		result[i] = updateSingleRating(ratings[i], opps)
	}

	return result
}

// ComputeRatingUpdates computes rating updates for match participants.
// botIDs, currentRatings, and scores must all have the same length.
func ComputeRatingUpdates(botIDs []string, currentRatings []Glicko2Rating, scores []float64) []RatingUpdate {
	if len(botIDs) != len(currentRatings) || len(botIDs) != len(scores) {
		return nil
	}

	newRatings := UpdateRatings(currentRatings, scores)
	updates := make([]RatingUpdate, len(botIDs))

	for i := range botIDs {
		updates[i] = RatingUpdate{
			BotID:                 botIDs[i],
			Mu:                    newRatings[i].Mu,
			Phi:                   newRatings[i].Phi,
			Sigma:                 newRatings[i].Sigma,
			DisplayRating:         newRatings[i].DisplayRating(),
			RatingMuBefore:        currentRatings[i].Mu,
			RatingPhiBefore:       currentRatings[i].Phi,
			RatingDeviationChange: newRatings[i].Phi - currentRatings[i].Phi,
		}
	}

	return updates
}
