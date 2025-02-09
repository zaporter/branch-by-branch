Lines starting with
--R: {filename} {completeness} indicate that at least completeness% of the file {filename} is required to begin attempting problems in this file.
--G: {name} starts a goal (where it is an example)
--T: {name} starts a goal where the goal is to add a new type of name {name}.
--E ends the last-started goal or type
--NOTE: {...} adds note to the last-started goal or type

Goal and type names do not have to be unique (though they probably should be).

(Goals and types are compiled to both be goals, but I just use T as a notation)


Theorems are added in-order so if you want parallelism, you can duplicate the file