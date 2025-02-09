import Corelib
--R: DataNatBasic.lean 100

--R: AlgebraGroupZeroOne.lean 100
--R: DataSubtype.lean 100
--R: OrderDefsLinearOrder.lean 100
--R: OrderNotation.lean 100

/- SUGGESTED COMMENT -/
/-!
# Basic definitions about `≤` and `<`

This file proves basic results about orders, provides extensive dot notation, defines useful order
classes and allows to transfer order instances.

## Type synonyms

* `OrderDual α` : A type synonym reversing the meaning of all inequalities, with notation `αᵒᵈ`.
* `AsLinearOrder α`: A type synonym to promote `PartialOrder α` to `LinearOrder α` using
  `IsTotal α (≤)`.

### Transferring orders

- `Order.Preimage`, `Preorder.lift`: Transfers a (pre)order on `β` to an order on `α`
  using a function `f : α → β`.
- `PartialOrder.lift`, `LinearOrder.lift`: Transfers a partial (resp., linear) order on `β` to a
  partial (resp., linear) order on `α` using an injective function `f`.

### Extra class

* `DenselyOrdered`: An order with no gap, i.e. for any two elements `a < b` there exists `c` such
  that `a < c < b`.

## Notes

`≤` and `<` are highly favored over `≥` and `>` in corelib. The reason is that we can formulate all
lemmas using `≤`/`<`, and `rw` has trouble unifying `≤` and `≥`. Hence choosing one direction spares
us useless duplication. This is enforced by a linter. See Note [nolint_ge] for more infos.

Dot notation is particularly useful on `≤` (`LE.le`) and `<` (`LT.lt`). To that end, we
provide many aliases to dot notation-less lemmas. For example, `le_trans` is aliased with
`LE.le.trans` and can be used to construct `hab.trans hbc : a ≤ c` when `hab : a ≤ b`,
`hbc : b ≤ c`, `lt_of_le_of_lt` is aliased as `LE.le.trans_lt` and can be used to construct
`hab.trans hbc : a < c` when `hab : a ≤ b`, `hbc : b < c`.

## Tags

preorder, order, partial order, poset, linear order, chain
-/
namespace Test_LogicBasic
open Function

variable {ι α β : Type*} {π : ι → Type*}

--G: preorder
section Preorder

variable [Preorder α] {a b c : α}

example : b ≤ c → a ≤ b → a ≤ c := le_trans'
example : b < c → a < b → a < c := lt_trans'
example : b ≤ c → a < b → a < c := lt_of_le_of_lt'
example : b < c → a ≤ b → a < c := lt_of_lt_of_le'

end Preorder
--E

--G: partialorder
section PartialOrder

variable [PartialOrder α] {a b : α}

example : a ≤ b → b ≤ a → b = a := ge_antisymm
example : a ≤ b → b ≠ a → a < b := lt_of_le_of_ne'
example : a ≠ b → a ≤ b → a < b := Ne.lt_of_le
example : b ≠ a → a ≤ b → a < b := Ne.lt_of_le'

end PartialOrder
--E

--G: self
section

variable [Preorder α] {a b c : α}

example(x : α) : x < x ↔ False :=
  lt_self_iff_false x

example : b ≤ c → a = b → a ≤ c :=
  le_of_le_of_eq'

example : b = c → a ≤ b → a ≤ c :=
  le_of_eq_of_le'

example : b < c → a = b → a < c :=
  lt_of_lt_of_eq'

example : b = c → a < b → a < c :=
  lt_of_eq_of_lt'

end
--E

--G: eq
section

variable [Preorder α] {x y : α}

/-- If `x = y` then `y ≤ x`. Note: this lemma uses `y ≤ x` instead of `x ≥ y`, because `le` is used
almost exclusively in corelib. -/
example : x = y → y ≤ x := Eq.ge

example : x = y → ¬x < y := fun h ↦ Eq.not_lt h

example : x = y → ¬y < x := fun h ↦ Eq.not_gt h

end
--E

--NOTE: Do rest of file

end Test_LogicBasic
