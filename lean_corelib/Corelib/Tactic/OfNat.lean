/-
Copyright (c) 2024 Eric Wieser. All rights reserved.
Released under Apache 2.0 license as described in the file LICENSE.
Authors: Eric Wieser
-/

/-! # The `ofNat()` macro -/

/--
This macro is a shorthand for `OfNat.ofNat` combined with `no_index`.

When writing lemmas about `OfNat.ofNat`, the term needs to be wrapped
in `no_index` so as not to confuse `simp`, as `no_index (OfNat.ofNat n)`.
-/
macro "ofNat(" n:term ")" : term => `(no_index (OfNat.ofNat $n))
