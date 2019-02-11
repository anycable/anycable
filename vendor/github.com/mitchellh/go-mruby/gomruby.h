// vim: ft=c ts=2 sts=2 st=2
/*
 * This header exists to simplify the headers that are included within
 * the Go files. This header should include all the necessary headers
 * for the compilation of the Go library.
 * */

#ifndef _GOMRUBY_H_INCLUDED
#define _GOMRUBY_H_INCLUDED

#include <errno.h>
#include <mruby.h>
#include <mruby/array.h>
#include <mruby/class.h>
#include <mruby/compile.h>
#include <mruby/error.h>
#include <mruby/irep.h>
#include <mruby/gc.h>
#include <mruby/hash.h>
#include <mruby/proc.h>
#include <mruby/string.h>
#include <mruby/throw.h>
#include <mruby/value.h>
#include <mruby/variable.h>

// (erikh) this can be set in mruby/mrbconfig.h so we can default it here.
// XXX I don't know how this actually plays out when the config is modified.
// I'm taking a WAG here. Either way, the default is 16 in vm.c.
#ifndef MRB_FUNCALL_ARGC_MAX
  #define MRB_FUNCALL_ARGC_MAX 16
#endif // MRB_FUNCALL_ARGC_MAX

//-------------------------------------------------------------------
// Helpers to deal with calling back into Go.
//-------------------------------------------------------------------
// This is declard in func.go and is a way for us to call back into
// Go to execute a method.
extern mrb_value goMRBFuncCall(mrb_state*, mrb_value);

// This method is used as a way to get a valid mrb_func_t that actually
// just calls back into Go.
static inline mrb_func_t _go_mrb_func_t() {
    return &goMRBFuncCall;
}

//-------------------------------------------------------------------
// Helpers to deal with calling into Ruby (C)
//-------------------------------------------------------------------
// These are some really horrible C macros that are used to wrap
// various mruby C API function calls so that we catch the exceptions.
// If we let exceptions through then the longjmp will cause a Go stack
// split.
#define GOMRUBY_EXC_PROTECT_START \
  struct mrb_jmpbuf *prev_jmp = mrb->jmp; \
  struct mrb_jmpbuf c_jmp; \
  mrb_value result = mrb_nil_value(); \
  MRB_TRY(&c_jmp) { \
    mrb->jmp = &c_jmp;

#define GOMRUBY_EXC_PROTECT_END \
    mrb->jmp = prev_jmp; \
  } MRB_CATCH(&c_jmp) { \
    mrb->jmp = prev_jmp; \
    result = mrb_nil_value();\
  } MRB_END_EXC(&c_jmp); \
  mrb_gc_protect(mrb, result); \
  return result;

static mrb_value _go_mrb_load_string(mrb_state *mrb, const char *s) {
  GOMRUBY_EXC_PROTECT_START
  result = mrb_load_string(mrb, s);
  GOMRUBY_EXC_PROTECT_END
}

static mrb_value _go_mrb_yield_argv(mrb_state *mrb, mrb_value b, mrb_int argc, const mrb_value *argv) {
  GOMRUBY_EXC_PROTECT_START
  result = mrb_yield_argv(mrb, b, argc, argv);
  GOMRUBY_EXC_PROTECT_END
}

static mrb_value _go_mrb_call(mrb_state *mrb, mrb_value b, mrb_sym method, mrb_int argc, const mrb_value *argv, mrb_value *block) {
  GOMRUBY_EXC_PROTECT_START
  if (block != NULL) {
    result = mrb_funcall_with_block(mrb, b, method, argc, argv, *block);
  } else {
    result = mrb_funcall_argv(mrb, b, method, argc, argv);
  }
  GOMRUBY_EXC_PROTECT_END
}

//-------------------------------------------------------------------
// Helpers to deal with getting arguments
//-------------------------------------------------------------------
// This is declard in args.go
extern void goGetArgAppend(mrb_value);

// This gets all arguments given to a function call and adds them to
// the accumulator in Go.
static inline int _go_mrb_get_args_all(mrb_state *s) {
  mrb_value *argv;
  mrb_value block;
  mrb_bool append;
  int argc, i;

  mrb_get_args(s, "*&?", &argv, &argc, &block, &append);

  for (i = 0; i < argc; i++) {
    goGetArgAppend(argv[i]);
  }

  if (append == FALSE || mrb_type(block) == MRB_TT_FALSE) {
    return argc;
  }

  argc++;
  goGetArgAppend(block);

  return argc;
}

//-------------------------------------------------------------------
// Misc. helpers
//-------------------------------------------------------------------

// This is used to help calculate the "send" value for the parser,
// since pointer arithmetic like this is hard in Go.
static inline const char *_go_mrb_calc_send(const char *s) {
  return s + strlen(s);
}

// Sets the capture_errors field on mrb_parser_state. Go can't access bit
// fields.
static inline void
_go_mrb_parser_set_capture_errors(struct mrb_parser_state *p, mrb_bool v) {
  p->capture_errors = v;
}

//-------------------------------------------------------------------
// Functions below here expose defines or inline functions that were
// otherwise inaccessible to Go directly.
//-------------------------------------------------------------------

static inline mrb_aspec _go_MRB_ARGS_ANY() {
  return MRB_ARGS_ANY();
}

static inline mrb_aspec _go_MRB_ARGS_ARG(int r, int o) {
  return MRB_ARGS_ARG(r, o);
}

static inline mrb_aspec _go_MRB_ARGS_BLOCK() {
  return MRB_ARGS_BLOCK();
}

static inline mrb_aspec _go_MRB_ARGS_NONE() {
  return MRB_ARGS_NONE();
}

static inline mrb_aspec _go_MRB_ARGS_OPT(int n) {
  return MRB_ARGS_OPT(n);
}

static inline mrb_aspec _go_MRB_ARGS_REQ(int n) {
  return MRB_ARGS_REQ(n);
}

static inline float _go_mrb_float(mrb_value o) {
  return mrb_float(o);
}

static inline int _go_mrb_fixnum(mrb_value o) {
  return mrb_fixnum(o);
}

static inline struct RBasic *_go_mrb_basic_ptr(mrb_value o) {
  return mrb_basic_ptr(o);
}

static inline struct RProc *_go_mrb_proc_ptr(mrb_value o) {
  return mrb_proc_ptr(o);
}

static inline enum mrb_vtype _go_mrb_type(mrb_value o) {
  return mrb_type(o);
}

static inline mrb_bool _go_mrb_nil_p(mrb_value o) {
  return mrb_nil_p(o);
}

static inline struct RClass *_go_mrb_class_ptr(mrb_value o) {
  return mrb_class_ptr(o);
}

static inline void _go_set_gc(mrb_state *m, int val) {
  mrb_gc *gc = &m->gc;
  gc->disabled = val;
}

static inline void _go_disable_gc(mrb_state *m) {
  _go_set_gc(m, 1);
}

static inline void _go_enable_gc(mrb_state *m) {
  _go_set_gc(m, 0);
}

static inline int _go_get_max_funcall_args() {
  return MRB_FUNCALL_ARGC_MAX;
}

// this function returns 1 if the value is dead, aka reaped or otherwise
// terminated by the GC.
static inline int _go_isdead(mrb_state *m, mrb_value o) {
  // immediate values such as Fixnums and symbols are never to be garbage
  // collected, so converting them to a basic pointer yields an invalid one.
  // This pattern is seen in the mruby source's gc.c.
  if mrb_immediate_p(o) {
    return 0;
  }

  struct RBasic *ptr = mrb_basic_ptr(o);

  // I don't actually know this is a potential condition but better safe than sorry.
  if (ptr == NULL) {
    return 1;
  }

  return mrb_object_dead_p(m, ptr);
}

static inline int _go_gc_live(mrb_state *m) {
  mrb_gc *gc = &m->gc;
  return gc->live;
}

static inline void _go_mrb_context_set_capture_errors(struct mrbc_context *ctx, int state) {
  ctx->capture_errors = FALSE;

  if (state != 0) {
    ctx->capture_errors = TRUE;
  }
}

static inline mrb_value _go_mrb_context_run(mrb_state *m, struct RProc *proc, mrb_value self, int *stack_keep) {
  mrb_value result = mrb_context_run(m, proc, self, *stack_keep);
  *stack_keep = proc->body.irep->nlocals;
  return result;
}

static inline struct RObject* _go_mrb_getobj(mrb_value v) {
  return mrb_obj_ptr(v);
}

static inline void _go_mrb_iv_set(mrb_state *m, mrb_value self, mrb_sym sym, mrb_value v) {
  mrb_iv_set(m, self, sym, v);
}

static inline mrb_value _go_mrb_iv_get(mrb_state *m, mrb_value self, mrb_sym sym) {
  return mrb_iv_get(m, self, sym);
}

static inline void _go_mrb_gv_set(mrb_state *m, mrb_sym sym, mrb_value v) {
  mrb_gv_set(m, sym, v);
}

static inline mrb_value _go_mrb_gv_get(mrb_state *m, mrb_sym sym) {
  return mrb_gv_get(m, sym);
}

#endif
