"""Optional host integration check: run generated code in installed Plymouth.
Uses no daemon, display, root privileges, or live boot files.
"""
import ctypes as C
import math
import sys

lib = C.CDLL(sys.argv[1])
P = C.c_void_p

class Result(C.Structure):
    _fields_ = [('kind', C.c_int), ('obj', P)]

class State(C.Structure):
    _fields_ = [('user_data', P), ('global_obj', P), ('local_obj', P), ('this', P)]

def fn(name, result, *args):
    f = getattr(lib, name)
    f.restype, f.argtypes = result, args
    return f

state = fn('script_state_new', P, P)(None)
fn('script_lib_math_setup', P, P)(state)
fn('script_lib_string_setup', P, P)(state)
fn('script_lib_image_setup', P, P, C.c_char_p)(state, sys.argv[3].encode())
displays = fn('ply_list_new', P)()
fn('script_lib_sprite_setup', P, P, P)(state, displays)
ply = fn('script_lib_plymouth_setup', P, P, C.c_int, C.c_int)(state, 0, 60)
op = fn('script_parse_file', P, C.c_char_p)(sys.argv[2].encode())
assert op, 'Generated script failed the installed Plymouth parser'
result = fn('script_execute', Result, P, P)(state, op)
assert result.kind == 0, ('script execution failed', result.kind)
global_obj = C.cast(state, C.POINTER(State)).contents.global_obj
get = fn('script_obj_hash_get_number', C.c_double, P, C.c_char_p)
refresh = fn('script_lib_plymouth_on_refresh', None, P, P)
for _ in range(60):
    refresh(state, ply)
assert math.isclose(get(global_obj, b'phase'), 10.0, abs_tol=1e-6), 'FPS timing was not executed'
fn('script_lib_plymouth_on_display_password', None, P, P, C.c_char_p, C.c_int)(state, ply, b'Password:', 4)
assert get(global_obj, b'dialog_active') == 1, 'password callback missing'
phase = get(global_obj, b'phase')
refresh(state, ply)
assert get(global_obj, b'phase') == phase, 'animation did not pause for password'
fn('script_lib_plymouth_on_display_question', None, P, P, C.c_char_p, C.c_char_p)(state, ply, b'Question:', b'answer')
fn('script_lib_plymouth_on_display_message', None, P, P, C.c_char_p)(state, ply, b'Boot message')
fn('script_lib_plymouth_on_hide_message', None, P, P, C.c_char_p)(state, ply, b'Boot message')
fn('script_lib_plymouth_on_display_normal', None, P, P)(state, ply)
assert get(global_obj, b'dialog_active') == 0, 'normal display did not resume'
for _ in range(60):
    refresh(state, ply)
assert math.isclose(get(global_obj, b'phase'), 8.0, abs_tol=1e-6), 'animation wrap did not execute'
fn('script_lib_plymouth_on_quit', None, P, P)(state, ply)
print('Installed Plymouth: parser, frame timing, wrap, password, question, message and quit callbacks passed')
