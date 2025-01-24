import Corelib.Data.Nat.Notation
import Corelib.Tactic.TypeStar
import Batteries.Classes.RatCast

/-!
# Basic definitions around the rational numbers

This file declares `ℚ` notation for the rationals and defines the nonnegative rationals `ℚ≥0`.
-/


notation "ℚ" => Rat

def NNRat := {q : ℚ // 0 ≤ q}

notation "ℚ≥0" => NNRat

class NNRatCast (K : Type*) where
  /-- The canonical homomorphism `ℚ≥0 → K`.

  Do not use directly. Use the coercion instead. -/
  protected nnratCast : ℚ≥0 → K

instance NNRat.instNNRatCast : NNRatCast ℚ≥0 where nnratCast q := q

variable {K : Type*} [NNRatCast K]

/-- Canonical homomorphism from `ℚ≥0` to a division semiring `K`.

This is just the bare function in order to aid in creating instances of `DivisionSemiring`. -/
@[coe, reducible, match_pattern] protected def NNRat.cast : ℚ≥0 → K := NNRatCast.nnratCast

-- See note [coercion into rings]
instance NNRatCast.toCoeHTCT [NNRatCast K] : CoeHTCT ℚ≥0 K where coe := NNRat.cast

instance Rat.instNNRatCast : NNRatCast ℚ := ⟨Subtype.val⟩

namespace NNRat

/-- The numerator of a nonnegative rational. -/
def num (q : ℚ≥0) : ℕ := (q : ℚ).num.natAbs

/-- The denominator of a nonnegative rational. -/
def den (q : ℚ≥0) : ℕ := (q : ℚ).den

@[simp] theorem num_mk (q : ℚ) (hq : 0 ≤ q) : num ⟨q, hq⟩ = q.num.natAbs := rfl
@[simp] theorem den_mk (q : ℚ) (hq : 0 ≤ q) : den ⟨q, hq⟩ = q.den := rfl

@[norm_cast] theorem cast_id (n : ℚ≥0) : NNRat.cast n = n := rfl
@[simp] theorem cast_eq_id : NNRat.cast = id := rfl

end NNRat

namespace Rat

@[norm_cast] theorem cast_id (n : ℚ) : Rat.cast n = n := rfl
@[simp] theorem cast_eq_id : Rat.cast = id := rfl

end Rat
