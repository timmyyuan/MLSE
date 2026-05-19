from __future__ import annotations

from typing import Any

GO_ABI_RUNTIME_IR = """@.file = private unnamed_addr constant [5 x i8] c"mlse\\00"
@.mismatch = private unnamed_addr constant [9 x i8] c"mismatch\\00"
@.panic = private unnamed_addr constant [6 x i8] c"panic\\00"
@.unsupported = private unnamed_addr constant [12 x i8] c"unsupported\\00"
@.assert_suffix = private unnamed_addr constant [11 x i8] c"assert.err\\00"
@.panic_suffix = private unnamed_addr constant [10 x i8] c"panic.err\\00"
@.model_suffix = private unnamed_addr constant [10 x i8] c"model.err\\00"

declare void @klee_make_symbolic(ptr, i64, ptr)
declare void @klee_report_error(ptr, i32, ptr, ptr)
declare ptr @malloc(i64)

define i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %alen = extractvalue { ptr, i64 } %a, 1
  %blen = extractvalue { ptr, i64 } %b, 1
  %same_len = icmp eq i64 %alen, %blen
  br i1 %same_len, label %loop, label %not_equal

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %done = icmp eq i64 %i, %alen
  br i1 %done, label %equal, label %body

body:
  %adata = extractvalue { ptr, i64 } %a, 0
  %bdata = extractvalue { ptr, i64 } %b, 0
  %aptr = getelementptr i8, ptr %adata, i64 %i
  %bptr = getelementptr i8, ptr %bdata, i64 %i
  %aval = load i8, ptr %aptr, align 1
  %bval = load i8, ptr %bptr, align 1
  %same_value = icmp eq i8 %aval, %bval
  br i1 %same_value, label %continue, label %not_equal

continue:
  %next = add i64 %i, 1
  br label %loop

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_slice_string_equal({ ptr, i64, i64 } %a, { ptr, i64, i64 } %b) {
entry:
  %alen = extractvalue { ptr, i64, i64 } %a, 1
  %blen = extractvalue { ptr, i64, i64 } %b, 1
  %same_len = icmp eq i64 %alen, %blen
  br i1 %same_len, label %loop, label %not_equal

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %done = icmp eq i64 %i, %alen
  br i1 %done, label %equal, label %body

body:
  %adata = extractvalue { ptr, i64, i64 } %a, 0
  %bdata = extractvalue { ptr, i64, i64 } %b, 0
  %aptr = getelementptr { ptr, i64 }, ptr %adata, i64 %i
  %bptr = getelementptr { ptr, i64 }, ptr %bdata, i64 %i
  %aval = load { ptr, i64 }, ptr %aptr, align 8
  %bval = load { ptr, i64 }, ptr %bptr, align 8
  %same_value = call i1 @__mlse_string_equal({ ptr, i64 } %aval, { ptr, i64 } %bval)
  br i1 %same_value, label %continue, label %not_equal

continue:
  %next = add i64 %i, 1
  br label %loop

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_slice_string_strict_equal({ ptr, i64, i64 } %a, { ptr, i64, i64 } %b) {
entry:
  %aptr = extractvalue { ptr, i64, i64 } %a, 0
  %bptr = extractvalue { ptr, i64, i64 } %b, 0
  %alen = extractvalue { ptr, i64, i64 } %a, 1
  %blen = extractvalue { ptr, i64, i64 } %b, 1
  %acap = extractvalue { ptr, i64, i64 } %a, 2
  %bcap = extractvalue { ptr, i64, i64 } %b, 2
  %anil = icmp eq ptr %aptr, null
  %bnil = icmp eq ptr %bptr, null
  %same_nil = icmp eq i1 %anil, %bnil
  %same_len = icmp eq i64 %alen, %blen
  %same_cap = icmp eq i64 %acap, %bcap
  %same_shape0 = and i1 %same_nil, %same_len
  %same_shape = and i1 %same_shape0, %same_cap
  br i1 %same_shape, label %compare_values, label %not_equal

compare_values:
  %same_values = call i1 @__mlse_slice_string_equal({ ptr, i64, i64 } %a, { ptr, i64, i64 } %b)
  ret i1 %same_values

not_equal:
  ret i1 false
}

define i1 @__mlse_error_equal(ptr %a, ptr %b) {
entry:
  %a_nil = icmp eq ptr %a, null
  %b_nil = icmp eq ptr %b, null
  %both_nil = and i1 %a_nil, %b_nil
  %same_nil = icmp eq i1 %a_nil, %b_nil
  br i1 %both_nil, label %equal, label %check_non_nil

check_non_nil:
  br i1 %same_nil, label %compare_message, label %not_equal

compare_message:
  %aval = load { ptr, i64 }, ptr %a, align 8
  %bval = load { ptr, i64 }, ptr %b, align 8
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %aval, { ptr, i64 } %bval)
  br i1 %same, label %equal, label %not_equal

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_ptr_i64_equal(ptr %a, ptr %b) {
entry:
  %a_nil = icmp eq ptr %a, null
  %b_nil = icmp eq ptr %b, null
  %both_nil = and i1 %a_nil, %b_nil
  %same_nil = icmp eq i1 %a_nil, %b_nil
  br i1 %both_nil, label %equal, label %check_non_nil

check_non_nil:
  br i1 %same_nil, label %compare_value, label %not_equal

compare_value:
  %aval = load i64, ptr %a, align 8
  %bval = load i64, ptr %b, align 8
  %same = icmp eq i64 %aval, %bval
  br i1 %same, label %equal, label %not_equal

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define { ptr, i64 } @runtime.add.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %adata = extractvalue { ptr, i64 } %a, 0
  %alen = extractvalue { ptr, i64 } %a, 1
  %bdata = extractvalue { ptr, i64 } %b, 0
  %blen = extractvalue { ptr, i64 } %b, 1
  %len = add i64 %alen, %blen
  %empty = icmp eq i64 %len, 0
  %alloc_len = select i1 %empty, i64 1, i64 %len
  %buf = call ptr @malloc(i64 %alloc_len)
  br label %copy_a

copy_a:
  %ai = phi i64 [ 0, %entry ], [ %anext, %copy_a_body ]
  %a_done = icmp eq i64 %ai, %alen
  br i1 %a_done, label %copy_b, label %copy_a_body

copy_a_body:
  %asrc = getelementptr i8, ptr %adata, i64 %ai
  %aval = load i8, ptr %asrc, align 1
  %adst = getelementptr i8, ptr %buf, i64 %ai
  store i8 %aval, ptr %adst, align 1
  %anext = add i64 %ai, 1
  br label %copy_a

copy_b:
  %bi = phi i64 [ 0, %copy_a ], [ %bnext, %copy_b_body ]
  %b_done = icmp eq i64 %bi, %blen
  br i1 %b_done, label %done, label %copy_b_body

copy_b_body:
  %bsrc = getelementptr i8, ptr %bdata, i64 %bi
  %bval = load i8, ptr %bsrc, align 1
  %offset = add i64 %alen, %bi
  %bdst = getelementptr i8, ptr %buf, i64 %offset
  store i8 %bval, ptr %bdst, align 1
  %bnext = add i64 %bi, 1
  br label %copy_b

done:
  %out0 = insertvalue { ptr, i64 } undef, ptr %buf, 0
  %out1 = insertvalue { ptr, i64 } %out0, i64 %len, 1
  ret { ptr, i64 } %out1
}

define i1 @runtime.eq.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b)
  ret i1 %same
}

define i1 @runtime.neq.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b)
  %not_same = xor i1 %same, true
  ret i1 %not_same
}

define ptr @runtime.any.box.string({ ptr, i64 } %value) {
entry:
  %box = call ptr @malloc(i64 16)
  store { ptr, i64 } %value, ptr %box, align 8
  ret ptr %box
}

define ptr @runtime.any.box.i64(i64 %value) {
entry:
  %box = call ptr @malloc(i64 8)
  store i64 %value, ptr %box, align 8
  ret ptr %box
}

define ptr @runtime.any.box.f64(double %value) {
entry:
  %box = call ptr @malloc(i64 8)
  store double %value, ptr %box, align 8
  ret ptr %box
}

define { ptr, i64 } @runtime.fmt.Sprintf({ ptr, i64 } %format, { ptr, i64, i64 } %args) {
entry:
  %fmt_ptr = extractvalue { ptr, i64 } %format, 0
  %fmt_len = extractvalue { ptr, i64 } %format, 1
  br label %scan

scan:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %after = add i64 %i, 1
  %has_next = icmp ult i64 %after, %fmt_len
  br i1 %has_next, label %check, label %fallback

check:
  %ch_ptr = getelementptr i8, ptr %fmt_ptr, i64 %i
  %next_ptr = getelementptr i8, ptr %fmt_ptr, i64 %after
  %ch = load i8, ptr %ch_ptr, align 1
  %next_ch = load i8, ptr %next_ptr, align 1
  %is_percent = icmp eq i8 %ch, 37
  %is_string = icmp eq i8 %next_ch, 115
  %found_string = and i1 %is_percent, %is_string
  br i1 %found_string, label %format_string, label %maybe_int

maybe_int:
  %is_int = icmp eq i8 %next_ch, 100
  %found_int = and i1 %is_percent, %is_int
  br i1 %found_int, label %format_i64, label %maybe_float

maybe_float:
  %float_last = add i64 %i, 4
  %has_float_width = icmp ult i64 %float_last, %fmt_len
  %maybe_float_format = and i1 %is_percent, %has_float_width
  br i1 %maybe_float_format, label %check_float_chars, label %maybe_unsupported

check_float_chars:
  %dot_ptr = getelementptr i8, ptr %fmt_ptr, i64 %after
  %one_index = add i64 %i, 2
  %f_index = add i64 %i, 3
  %k_index = add i64 %i, 4
  %one_ptr = getelementptr i8, ptr %fmt_ptr, i64 %one_index
  %f_ptr = getelementptr i8, ptr %fmt_ptr, i64 %f_index
  %k_ptr = getelementptr i8, ptr %fmt_ptr, i64 %k_index
  %dot_ch = load i8, ptr %dot_ptr, align 1
  %one_ch = load i8, ptr %one_ptr, align 1
  %f_ch = load i8, ptr %f_ptr, align 1
  %k_ch = load i8, ptr %k_ptr, align 1
  %is_dot = icmp eq i8 %dot_ch, 46
  %is_one = icmp eq i8 %one_ch, 49
  %is_f = icmp eq i8 %f_ch, 102
  %is_k = icmp eq i8 %k_ch, 75
  %float_a = and i1 %is_dot, %is_one
  %float_b = and i1 %is_f, %is_k
  %found_float = and i1 %float_a, %float_b
  br i1 %found_float, label %format_f64, label %unsupported

maybe_unsupported:
  br i1 %is_percent, label %unsupported, label %continue

continue:
  %next = add i64 %i, 1
  br label %scan

format_string:
  %args_len = extractvalue { ptr, i64, i64 } %args, 1
  %has_arg = icmp ugt i64 %args_len, 0
  br i1 %has_arg, label %arg_ok, label %fallback

arg_ok:
  %args_data = extractvalue { ptr, i64, i64 } %args, 0
  %arg_slot = getelementptr ptr, ptr %args_data, i64 0
  %boxed_arg = load ptr, ptr %arg_slot, align 8
  %arg = load { ptr, i64 }, ptr %boxed_arg, align 8
  %prefix0 = insertvalue { ptr, i64 } undef, ptr %fmt_ptr, 0
  %prefix1 = insertvalue { ptr, i64 } %prefix0, i64 %i, 1
  %prefix_arg = call { ptr, i64 } @runtime.add.string({ ptr, i64 } %prefix1, { ptr, i64 } %arg)
  %suffix_index = add i64 %i, 2
  %suffix_ptr = getelementptr i8, ptr %fmt_ptr, i64 %suffix_index
  %suffix_len = sub i64 %fmt_len, %suffix_index
  %suffix0 = insertvalue { ptr, i64 } undef, ptr %suffix_ptr, 0
  %suffix1 = insertvalue { ptr, i64 } %suffix0, i64 %suffix_len, 1
  %full = call { ptr, i64 } @runtime.add.string({ ptr, i64 } %prefix_arg, { ptr, i64 } %suffix1)
  ret { ptr, i64 } %full

format_i64:
  %args_len_i64 = extractvalue { ptr, i64, i64 } %args, 1
  %has_i64_arg = icmp ugt i64 %args_len_i64, 0
  br i1 %has_i64_arg, label %i64_arg_ok, label %fallback

i64_arg_ok:
  %args_data_i64 = extractvalue { ptr, i64, i64 } %args, 0
  %arg_slot_i64 = getelementptr ptr, ptr %args_data_i64, i64 0
  %boxed_arg_i64 = load ptr, ptr %arg_slot_i64, align 8
  %arg_i64 = load i64, ptr %boxed_arg_i64, align 8
  %buf_i64 = call ptr @malloc(i64 9)
  store i8 100, ptr %buf_i64, align 1
  %value_ptr_i64 = getelementptr i8, ptr %buf_i64, i64 1
  store i64 %arg_i64, ptr %value_ptr_i64, align 1
  %out_i64_0 = insertvalue { ptr, i64 } undef, ptr %buf_i64, 0
  %out_i64_1 = insertvalue { ptr, i64 } %out_i64_0, i64 9, 1
  ret { ptr, i64 } %out_i64_1

format_f64:
  %args_len_f64 = extractvalue { ptr, i64, i64 } %args, 1
  %has_f64_arg = icmp ugt i64 %args_len_f64, 0
  br i1 %has_f64_arg, label %f64_arg_ok, label %fallback

f64_arg_ok:
  %args_data_f64 = extractvalue { ptr, i64, i64 } %args, 0
  %arg_slot_f64 = getelementptr ptr, ptr %args_data_f64, i64 0
  %boxed_arg_f64 = load ptr, ptr %arg_slot_f64, align 8
  %arg_f64 = load double, ptr %boxed_arg_f64, align 8
  %buf_f64 = call ptr @malloc(i64 9)
  store i8 102, ptr %buf_f64, align 1
  %value_ptr_f64 = getelementptr i8, ptr %buf_f64, i64 1
  store double %arg_f64, ptr %value_ptr_f64, align 1
  %out_f64_0 = insertvalue { ptr, i64 } undef, ptr %buf_f64, 0
  %out_f64_1 = insertvalue { ptr, i64 } %out_f64_0, i64 9, 1
  ret { ptr, i64 } %out_f64_1

fallback:
  ret { ptr, i64 } %format

unsupported:
  call void @klee_report_error(ptr @.file, i32 3, ptr @.unsupported, ptr @.model_suffix)
  unreachable
}

define ptr @runtime.errors.New({ ptr, i64 } %message) {
entry:
  %err = call ptr @malloc(i64 16)
  store { ptr, i64 } %message, ptr %err, align 8
  ret ptr %err
}

define ptr @runtime.fmt.Errorf({ ptr, i64 } %format, { ptr, i64, i64 } %args) {
entry:
  %args_len = extractvalue { ptr, i64, i64 } %args, 1
  %no_args = icmp eq i64 %args_len, 0
  br i1 %no_args, label %constant_error, label %unsupported

constant_error:
  %err = call ptr @runtime.errors.New({ ptr, i64 } %format)
  ret ptr %err

unsupported:
  call void @klee_report_error(ptr @.file, i32 4, ptr @.unsupported, ptr @.model_suffix)
  unreachable
}

define { ptr, i64, i64 } @runtime.makeslice(i64 %len, i64 %cap) {
entry:
  %bytes = mul i64 %cap, 8
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue { ptr, i64, i64 } undef, ptr %buf, 0
  %slice1 = insertvalue { ptr, i64, i64 } %slice0, i64 %len, 1
  %slice2 = insertvalue { ptr, i64, i64 } %slice1, i64 %cap, 2
  ret { ptr, i64, i64 } %slice2
}

define { ptr, i64, i64 } @runtime.growslice(ptr %data, i64 %new_len, i64 %old_cap, i64 %count, i64 %elem_size) {
entry:
  %bytes = mul i64 %new_len, %elem_size
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue { ptr, i64, i64 } undef, ptr %buf, 0
  %slice1 = insertvalue { ptr, i64, i64 } %slice0, i64 %new_len, 1
  %slice2 = insertvalue { ptr, i64, i64 } %slice1, i64 %new_len, 2
  ret { ptr, i64, i64 } %slice2
}

define ptr @runtime.newobject(i64 %size, i64 %align) {
entry:
  %empty = icmp eq i64 %size, 0
  %alloc_len = select i1 %empty, i64 1, i64 %size
  %obj = call ptr @malloc(i64 %alloc_len)
  ret ptr %obj
}

define ptr @runtime.composite.map({ ptr, i64 } %first, { ptr, i64 } %second, { ptr, i64 } %third) {
entry:
  %map = call ptr @malloc(i64 8)
  ret ptr %map
}

define ptr @runtime.newobject__sig2(i64 %size, i64 %align) {
entry:
  %obj = call ptr @runtime.newobject(i64 %size, i64 %align)
  ret ptr %obj
}

define i64 @__mlse_string_slice_len(ptr %slice) {
entry:
  %is_null = icmp eq ptr %slice, null
  br i1 %is_null, label %zero, label %load_len

load_len:
  %len = load i64, ptr %slice, align 8
  ret i64 %len

zero:
  ret i64 0
}

define ptr @__mlse_string_slice_index(ptr %slice, i64 %index) {
entry:
  %data_slot = getelementptr i8, ptr %slice, i64 8
  %data = load ptr, ptr %data_slot, align 8
  %elem = getelementptr { ptr, i64 }, ptr %data, i64 %index
  ret ptr %elem
}

define { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %old_data = extractvalue { ptr, i64, i64 } %slice, 0
  %old_len = extractvalue { ptr, i64, i64 } %slice, 1
  %new_len = add i64 %old_len, 1
  %bytes = mul i64 %new_len, 16
  %buf = call ptr @malloc(i64 %bytes)
  br label %copy

copy:
  %i = phi i64 [ 0, %entry ], [ %next, %copy_body ]
  %done = icmp eq i64 %i, %old_len
  br i1 %done, label %append, label %copy_body

copy_body:
  %src = getelementptr { ptr, i64 }, ptr %old_data, i64 %i
  %val = load { ptr, i64 }, ptr %src, align 8
  %dst = getelementptr { ptr, i64 }, ptr %buf, i64 %i
  store { ptr, i64 } %val, ptr %dst, align 8
  %next = add i64 %i, 1
  br label %copy

append:
  %elem_value = load { ptr, i64 }, ptr %elem, align 8
  %tail = getelementptr { ptr, i64 }, ptr %buf, i64 %old_len
  store { ptr, i64 } %elem_value, ptr %tail, align 8
  %out0 = insertvalue { ptr, i64, i64 } undef, ptr %buf, 0
  %out1 = insertvalue { ptr, i64, i64 } %out0, i64 %new_len, 1
  %out2 = insertvalue { ptr, i64, i64 } %out1, i64 %new_len, 2
  ret { ptr, i64, i64 } %out2
}

define void @runtime.panic.index(i64 %index, i64 %len) {
entry:
  call void @klee_report_error(ptr @.file, i32 1, ptr @.panic, ptr @.panic_suffix)
  unreachable
}
"""


GO_ABI_MOTUS_VOID_RUNTIME_IR = """
define ptr @runtime.any.box.ptr.pair(ptr %value) {
entry:
  %box = call ptr @malloc(i64 8)
  store ptr %value, ptr %box, align 8
  ret ptr %box
}
"""


GO_ABI_SIMPLE_PTR_MAP_RUNTIME_IR = """
define ptr @runtime.make.map() {
entry:
  %map = call ptr @malloc(i64 8)
  ret ptr %map
}

define ptr @runtime.index.map(ptr %map, { ptr, i64 } %key) {
entry:
  ret ptr %map
}

define ptr @runtime.store.index.map(ptr %map, { ptr, i64 } %key, ptr %value) {
entry:
  ret ptr %map
}

define ptr @runtime.store.index.map__sig2(ptr %map, { ptr, i64 } %key, ptr %value) {
entry:
  ret ptr %map
}

define i1 @runtime.index.value__sig2(ptr %map, { ptr, i64 } %key) {
entry:
  ret i1 false
}

define ptr @runtime.store.index.value(ptr %map, { ptr, i64 } %key, i1 %value) {
entry:
  ret ptr %map
}

define ptr @VRegionMap() {
entry:
  %map = call ptr @malloc(i64 8)
  ret ptr %map
}
"""


GO_ABI_MAP_STRING_STRING_RUNTIME_IR = """
@__mlse_custom_go_env_bytes = private unnamed_addr constant [21 x i8] c"CUSTOM_LEGO_BUILD_ENV"

define { ptr, i64 } @customGoEnv() {
entry:
  %out0 = insertvalue { ptr, i64 } undef, ptr @__mlse_custom_go_env_bytes, 0
  %out1 = insertvalue { ptr, i64 } %out0, i64 21, 1
  ret { ptr, i64 } %out1
}

define ptr @__mlse_map_string_string_new() {
entry:
  %map = call ptr @malloc(i64 48)
  store i64 0, ptr %map, align 8
  %present_slot = getelementptr i8, ptr %map, i64 8
  store i8 0, ptr %present_slot, align 1
  %key_slot = getelementptr i8, ptr %map, i64 16
  store { ptr, i64 } zeroinitializer, ptr %key_slot, align 8
  %value_slot = getelementptr i8, ptr %map, i64 32
  store { ptr, i64 } zeroinitializer, ptr %value_slot, align 8
  ret ptr %map
}

define ptr @runtime.store.index.map(ptr %map, { ptr, i64 } %key, { ptr, i64 } %value) {
entry:
  %is_nil = icmp eq ptr %map, null
  br i1 %is_nil, label %panic, label %check_slot

check_slot:
  %present_slot = getelementptr i8, ptr %map, i64 8
  %present_raw = load i8, ptr %present_slot, align 1
  %present = icmp ne i8 %present_raw, 0
  br i1 %present, label %maybe_update, label %insert

maybe_update:
  %old_key_slot = getelementptr i8, ptr %map, i64 16
  %old_key = load { ptr, i64 }, ptr %old_key_slot, align 8
  %same_key = call i1 @__mlse_string_equal({ ptr, i64 } %old_key, { ptr, i64 } %key)
  br i1 %same_key, label %write_value, label %unsupported

insert:
  store i64 1, ptr %map, align 8
  store i8 1, ptr %present_slot, align 1
  %new_key_slot = getelementptr i8, ptr %map, i64 16
  store { ptr, i64 } %key, ptr %new_key_slot, align 8
  br label %write_value

write_value:
  %value_slot = getelementptr i8, ptr %map, i64 32
  store { ptr, i64 } %value, ptr %value_slot, align 8
  ret ptr %map

panic:
  call void @klee_report_error(ptr @.file, i32 5, ptr @.panic, ptr @.panic_suffix)
  unreachable

unsupported:
  call void @klee_report_error(ptr @.file, i32 6, ptr @.unsupported, ptr @.model_suffix)
  unreachable
}

define i1 @__mlse_map_string_string_equal(ptr %a, ptr %b) {
entry:
  %a_nil = icmp eq ptr %a, null
  %b_nil = icmp eq ptr %b, null
  %both_nil = and i1 %a_nil, %b_nil
  %same_nil = icmp eq i1 %a_nil, %b_nil
  br i1 %both_nil, label %equal, label %check_non_nil

check_non_nil:
  br i1 %same_nil, label %compare_header, label %not_equal

compare_header:
  %a_len = load i64, ptr %a, align 8
  %b_len = load i64, ptr %b, align 8
  %same_len = icmp eq i64 %a_len, %b_len
  br i1 %same_len, label %compare_present, label %not_equal

compare_present:
  %a_present_slot = getelementptr i8, ptr %a, i64 8
  %b_present_slot = getelementptr i8, ptr %b, i64 8
  %a_present_raw = load i8, ptr %a_present_slot, align 1
  %b_present_raw = load i8, ptr %b_present_slot, align 1
  %same_present = icmp eq i8 %a_present_raw, %b_present_raw
  %present = icmp ne i8 %a_present_raw, 0
  br i1 %same_present, label %maybe_values, label %not_equal

maybe_values:
  br i1 %present, label %compare_values, label %equal

compare_values:
  %a_key_slot = getelementptr i8, ptr %a, i64 16
  %b_key_slot = getelementptr i8, ptr %b, i64 16
  %a_key = load { ptr, i64 }, ptr %a_key_slot, align 8
  %b_key = load { ptr, i64 }, ptr %b_key_slot, align 8
  %same_key = call i1 @__mlse_string_equal({ ptr, i64 } %a_key, { ptr, i64 } %b_key)
  br i1 %same_key, label %compare_map_values, label %not_equal

compare_map_values:
  %a_value_slot = getelementptr i8, ptr %a, i64 32
  %b_value_slot = getelementptr i8, ptr %b, i64 32
  %a_value = load { ptr, i64 }, ptr %a_value_slot, align 8
  %b_value = load { ptr, i64 }, ptr %b_value_slot, align 8
  %same_value = call i1 @__mlse_string_equal({ ptr, i64 } %a_value, { ptr, i64 } %b_value)
  br i1 %same_value, label %equal, label %not_equal

equal:
  ret i1 true

not_equal:
  ret i1 false
}
"""


GO_ABI_MOTUS_MOD12_RUNTIME_IR = """
@__mlse_mod12_psm0_bytes = private unnamed_addr constant [1 x i8] c"p"
@__mlse_mod12_psm1_bytes = private unnamed_addr constant [1 x i8] c"q"
@__mlse_mod12_psm_data = global [2 x { ptr, i64 }] [{ ptr, i64 } { ptr @__mlse_mod12_psm0_bytes, i64 1 }, { ptr, i64 } { ptr @__mlse_mod12_psm1_bytes, i64 1 }]
@__mlse_mod12_psm_slice = global { i64, ptr } { i64 2, ptr @__mlse_mod12_psm_data }
@__mlse_mod12_psm_slot = global ptr @__mlse_mod12_psm_slice
@__mlse_mod12_zero_i64 = global i64 0
@__mlse_mod12_true_i1 = global i1 true

define ptr @example.com.smtcmpmod12.dal.GetCDSConfKey({ ptr, i64 } %plugin) {
entry:
  ret ptr null
}

define ptr @example.com.smtcmpmod12.dal.GetKeyStatInfo(ptr %ctx, ptr %key) {
entry:
  ret ptr @__mlse_mod12_psm_slot
}

define void @example.com.smtcmpmod12.logs.CtxError(ptr %ctx, { ptr, i64 } %format, ptr %err, { ptr, i64 } %plugin) {
entry:
  ret void
}

define ptr @example.com.smtcmpmod12.sets.NewStringSetFromSlice({ ptr, i64, i64 } %items) {
entry:
  ret ptr null
}

define ptr @runtime.field.addr.PSM(ptr %value) {
entry:
  ret ptr @__mlse_mod12_psm_slot
}

define i64 @runtime.range.len.value(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @runtime.index.value(ptr %slice, i64 %index) {
entry:
  %elem = call ptr @__mlse_string_slice_index(ptr %slice, i64 %index)
  ret ptr %elem
}

define i64 @runtime.convert.result.to.i64(ptr %value) {
entry:
  %out = load i64, ptr %value, align 8
  ret i64 %out
}

define i1 @runtime.convert.result.to.bool(ptr %value) {
entry:
  %out = load i1, ptr %value, align 1
  ret i1 %out
}

define i64 @__mlse_old_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define i64 @__mlse_new_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @__mlse_old_diffcase_Len(ptr %set) {
entry:
  ret ptr @__mlse_mod12_zero_i64
}

define ptr @__mlse_new_diffcase_Len(ptr %set) {
entry:
  ret ptr @__mlse_mod12_zero_i64
}

define ptr @__mlse_old_diffcase_Contains(ptr %set, ptr %item) {
entry:
  ret ptr @__mlse_mod12_true_i1
}

define ptr @__mlse_new_diffcase_Contains(ptr %set, ptr %item) {
entry:
  ret ptr @__mlse_mod12_true_i1
}

define { ptr, i64, i64 } @__mlse_old_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}

define { ptr, i64, i64 } @__mlse_new_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}
"""


GO_ABI_MOTUS_MOD29_RUNTIME_IR = """
@__mlse_mod29_empty_slice = global { i64, ptr } { i64 0, ptr null }
@__mlse_mod29_plugins_slot = global ptr @__mlse_mod29_empty_slice
@__mlse_mod29_zero_result = global i64 0

define ptr @runtime.field.addr.Plugins(ptr %request) {
entry:
  ret ptr @__mlse_mod29_plugins_slot
}

define i64 @runtime.range.len.value(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @runtime.index.value(ptr %slice, i64 %index) {
entry:
  %elem = call ptr @__mlse_string_slice_index(ptr %slice, i64 %index)
  ret ptr %elem
}

define ptr @runtime.zero.result() {
entry:
  ret ptr null
}

define i1 @runtime.neq.result(ptr %a, ptr %b) {
entry:
  %same = icmp eq ptr %a, %b
  %neq = xor i1 %same, true
  ret i1 %neq
}

define i64 @__mlse_old_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define i64 @__mlse_new_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @__mlse_old_diffcase_GetByPluginName(ptr %plugin_info, ptr %name) {
entry:
  ret ptr null
}

define ptr @__mlse_new_diffcase_GetByPluginName(ptr %plugin_info, ptr %name) {
entry:
  ret ptr null
}

define { ptr, i64, i64 } @__mlse_old_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}

define { ptr, i64, i64 } @__mlse_new_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}
"""


GO_ABI_MOTUS_MOD30_RUNTIME_IR = """
@__mlse_mod30_plugin0_bytes = private unnamed_addr constant [1 x i8] c"p"
@__mlse_mod30_owner0_bytes = private unnamed_addr constant [1 x i8] c"o"
@__mlse_mod30_plugins_data = global [1 x { ptr, i64 }] [{ ptr, i64 } { ptr @__mlse_mod30_plugin0_bytes, i64 1 }]
@__mlse_mod30_owners_data = global [1 x { ptr, i64 }] [{ ptr, i64 } { ptr @__mlse_mod30_owner0_bytes, i64 1 }]
@__mlse_mod30_plugins_slice = global { i64, ptr } { i64 1, ptr @__mlse_mod30_plugins_data }
@__mlse_mod30_owners_slice = global { i64, ptr } { i64 1, ptr @__mlse_mod30_owners_data }
@__mlse_mod30_plugins_slot = global ptr @__mlse_mod30_plugins_slice
@__mlse_mod30_owners_slot = global ptr @__mlse_mod30_owners_slice
@__mlse_mod30_get_error = global i64 1

define ptr @example.com.smtcmpmod30.binding.BindAndValidate(ptr %ctx, ptr %request) {
entry:
  ret ptr null
}

define void @example.com.smtcmpmod30.logs.CtxWarn(ptr %ctx, { ptr, i64 } %format, ptr %err) {
entry:
  ret void
}

define void @example.com.smtcmpmod30.response.BadRequest(ptr %ctx, { ptr, i64 } %message) {
entry:
  ret void
}

define void @example.com.smtcmpmod30.response.OK(ptr %ctx, ptr %value) {
entry:
  ret void
}

define ptr @runtime.field.addr.Plugins(ptr %request) {
entry:
  ret ptr @__mlse_mod30_plugins_slot
}

define ptr @runtime.field.addr.Owners(ptr %request) {
entry:
  ret ptr @__mlse_mod30_owners_slot
}

define i64 @runtime.range.len.value(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @runtime.index.value(ptr %slice, i64 %index) {
entry:
  %elem = call ptr @__mlse_string_slice_index(ptr %slice, i64 %index)
  ret ptr %elem
}

define ptr @runtime.zero.result() {
entry:
  ret ptr null
}

define i1 @runtime.neq.result(ptr %a, ptr %b) {
entry:
  %same = icmp eq ptr %a, %b
  %neq = xor i1 %same, true
  ret i1 %neq
}

define i64 @__mlse_old_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define i64 @__mlse_new_diffcase_len(ptr %slice) {
entry:
  %len = call i64 @__mlse_string_slice_len(ptr %slice)
  ret i64 %len
}

define ptr @__mlse_old_diffcase_GetByPluginName(ptr %plugin_info, ptr %name) {
entry:
  ret ptr @__mlse_mod30_get_error
}

define ptr @__mlse_new_diffcase_GetByPluginName(ptr %plugin_info, ptr %name) {
entry:
  ret ptr @__mlse_mod30_get_error
}

define { ptr, i64, i64 } @__mlse_old_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}

define { ptr, i64, i64 } @__mlse_new_diffcase_append({ ptr, i64, i64 } %slice, ptr %elem) {
entry:
  %out = call { ptr, i64, i64 } @__mlse_string_slice_append({ ptr, i64, i64 } %slice, ptr %elem)
  ret { ptr, i64, i64 } %out
}
"""


GO_ABI_MOTUS_MOD34_RUNTIME_IR = """
@__mlse_default_product_id = global i64 1

define ptr @defaultProductID() {
entry:
  ret ptr @__mlse_default_product_id
}

define i1 @runtime.type.assert.any.to.bool(ptr %value) {
entry:
  ret i1 false
}

define ptr @runtime.zero.any() {
entry:
  ret ptr null
}
"""


def go_abi_extra_runtime_ir(metadata: dict[str, Any]) -> str:
    name = metadata.get("name", "")
    pieces: list[str] = []
    if name == "motus-mod14-foo1-foo2":
        pieces.append(
            """
@__mlse_global_input_value = global i64 0

define i64 @GlobalInput() {
entry:
  %value = load i64, ptr @__mlse_global_input_value, align 8
  ret i64 %value
}
"""
        )
    if name == "motus-mod26-wrapcompileenv1-wrapcompileenv2":
        pieces.append(
            """
define ptr @runtime.index.value(ptr %map, { ptr, i64 } %key) {
entry:
  ret ptr null
}

define { ptr, i64 } @runtime.convert.value.to.string(ptr %value) {
entry:
  ret { ptr, i64 } zeroinitializer
}
"""
        )
    if name in {
        "motus-mod27-getpsmrelatedpluginlist1-getpsmrelatedpluginlist2",
        "motus-mod28-getpsmrelatedpluginlist1-getpsmrelatedpluginlist2",
    }:
        pieces.append(GO_ABI_SIMPLE_PTR_MAP_RUNTIME_IR)
        pieces.append(
            """
define { ptr, i64 } @runtime.index.value(ptr %map, { ptr, i64 } %key) {
entry:
  ret { ptr, i64 } %key
}

define i1 @runtime.strings.Contains({ ptr, i64 } %haystack, { ptr, i64 } %needle) {
entry:
  ret i1 true
}

define { ptr, i64, i64 } @runtime.strings.Split({ ptr, i64 } %text, { ptr, i64 } %sep) {
entry:
  %sep_ptr = extractvalue { ptr, i64 } %sep, 0
  %sep_len = extractvalue { ptr, i64 } %sep, 1
  %has_sep = icmp ugt i64 %sep_len, 0
  br i1 %has_sep, label %read_sep, label %single

read_sep:
  %sep_ch = load i8, ptr %sep_ptr, align 1
  %is_slash = icmp eq i8 %sep_ch, 47
  br i1 %is_slash, label %slash, label %single

single:
  %single_buf = call ptr @malloc(i64 16)
  store { ptr, i64 } %text, ptr %single_buf, align 8
  %single0 = insertvalue { ptr, i64, i64 } undef, ptr %single_buf, 0
  %single1 = insertvalue { ptr, i64, i64 } %single0, i64 1, 1
  %single2 = insertvalue { ptr, i64, i64 } %single1, i64 1, 2
  ret { ptr, i64, i64 } %single2

slash:
  %slash_buf = call ptr @malloc(i64 48)
  store { ptr, i64 } zeroinitializer, ptr %slash_buf, align 8
  %slot1 = getelementptr { ptr, i64 }, ptr %slash_buf, i64 1
  store { ptr, i64 } zeroinitializer, ptr %slot1, align 8
  %slot2 = getelementptr { ptr, i64 }, ptr %slash_buf, i64 2
  store { ptr, i64 } %text, ptr %slot2, align 8
  %slash0 = insertvalue { ptr, i64, i64 } undef, ptr %slash_buf, 0
  %slash1 = insertvalue { ptr, i64, i64 } %slash0, i64 3, 1
  %slash2 = insertvalue { ptr, i64, i64 } %slash1, i64 3, 2
  ret { ptr, i64, i64 } %slash2
}
"""
        )
    if name in {
        "motus-mod18-foo1-foo2",
        "motus-mod20-foo1-foo2",
    }:
        pieces.append(GO_ABI_MOTUS_VOID_RUNTIME_IR)
    if name == "motus-mod19-foo1-foo2":
        pieces.append(GO_ABI_MAP_STRING_STRING_RUNTIME_IR)
    if name == "motus-mod12-getallpsm1-getallpsm2":
        pieces.append(GO_ABI_MOTUS_MOD12_RUNTIME_IR)
    if name == "motus-mod29-addowners1-addowners2":
        pieces.append(GO_ABI_MOTUS_MOD29_RUNTIME_IR)
    if name == "motus-mod30-addowners1-addowners2":
        pieces.append(GO_ABI_MOTUS_MOD30_RUNTIME_IR)
    if name == "motus-mod34-getadddefaultfactiontype1-getadddefaultfactiontype2":
        pieces.append(GO_ABI_MOTUS_MOD34_RUNTIME_IR)
    return "\n".join(piece.strip() for piece in pieces if piece.strip())
