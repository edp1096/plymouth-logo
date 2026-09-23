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
scene = len(sys.argv) > 4 and sys.argv[4] == 'scene'
count = 3 if scene else int(sys.argv[4]) if len(sys.argv) > 4 else 1
global_obj = C.cast(state, C.POINTER(State)).contents.global_obj
get = fn('script_obj_hash_get_number', C.c_double, P, C.c_char_p)
refresh = fn('script_lib_plymouth_on_refresh', None, P, P)
if len(sys.argv) > 4:
    for i in range(count):
        assert get(global_obj, ('loaded_width_'+str(i)).encode()) == 80, 'nested frame path did not load'
        assert get(global_obj, ('loaded_height_'+str(i)).encode()) == 40, 'nested frame dimensions incorrect'
for _ in range(60):
    refresh(state, ply)
for i, expected in enumerate(([0.0, 10.0, 0.0] if scene else [10.0, 7.0][:count])):
    assert math.isclose(get(global_obj, ('phase_'+str(i)).encode()), expected, abs_tol=1e-6), 'independent FPS timing failed'
fn('script_lib_plymouth_on_display_password', None, P, P, C.c_char_p, C.c_int)(state, ply, b'Password:', 4)
assert get(global_obj, b'dialog_active') == 1, 'password callback missing'
phases = [get(global_obj, ('phase_'+str(i)).encode()) for i in range(count)]
refresh(state, ply)
assert phases == [get(global_obj, ('phase_'+str(i)).encode()) for i in range(count)], 'animations did not pause for password'
fn('script_lib_plymouth_on_display_question', None, P, P, C.c_char_p, C.c_char_p)(state, ply, b'Question:', b'answer')
fn('script_lib_plymouth_on_display_message', None, P, P, C.c_char_p)(state, ply, b'Boot message')
fn('script_lib_plymouth_on_hide_message', None, P, P, C.c_char_p)(state, ply, b'Boot message')
fn('script_lib_plymouth_on_display_normal', None, P, P)(state, ply)
assert get(global_obj, b'dialog_active') == 0, 'normal display did not resume'
for _ in range(60):
    refresh(state, ply)
for i, expected in enumerate(([0.0, 8.0, 0.0] if scene else [8.0, 5.0][:count])):
    assert math.isclose(get(global_obj, ('phase_'+str(i)).encode()), expected, abs_tol=1e-6), 'independent animation wrap failed'
fn('script_lib_plymouth_on_quit', None, P, P)(state, ply)
phases = [get(global_obj, ('phase_'+str(i)).encode()) for i in range(count)]
refresh(state, ply)
assert phases == [get(global_obj, ('phase_'+str(i)).encode()) for i in range(count)], 'quit did not stop animations'
print('Installed Plymouth: parser, frame timing, wrap, password, question, message and quit callbacks passed')
