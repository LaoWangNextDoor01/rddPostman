# 把 rddPostman 与 Postman 的窗口顶边对齐、左右并排，方便对比标题栏高度。
#
# 用法：
#   1. 先启动 rddPostman（IDE 里跑就行）和 Postman；
#   2. 运行：python scripts/align-vs-postman.py
#
# 本脚本只移动窗口位置，不改变任何窗口尺寸，也不启动/关闭任何应用。
import ctypes
import time
from ctypes import wintypes

u = ctypes.windll.user32
k = ctypes.windll.kernel32
u.SetProcessDPIAware()

k.OpenProcess.restype = wintypes.HANDLE
k.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
k.QueryFullProcessImageNameW.restype = wintypes.BOOL
k.QueryFullProcessImageNameW.argtypes = [
    wintypes.HANDLE, wintypes.DWORD, ctypes.c_wchar_p,
    ctypes.POINTER(wintypes.DWORD),
]


class RECT(ctypes.Structure):
    _fields_ = [('Left', ctypes.c_long), ('Top', ctypes.c_long),
                ('Right', ctypes.c_long), ('Bottom', ctypes.c_long)]


def rect(h):
    r = RECT()
    u.GetWindowRect(h, ctypes.byref(r))
    return r


def process_image(pid):
    """返回进程 exe 的完整路径（小写），失败返回空串。"""
    PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
    h = k.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, False, pid)
    if not h:
        return ''
    try:
        size = wintypes.DWORD(1024)
        buf = ctypes.create_unicode_buffer(size.value)
        ok = k.QueryFullProcessImageNameW(h, 0, buf, ctypes.byref(size))
        return buf.value.lower() if ok else ''
    finally:
        k.CloseHandle(h)


def find_windows():
    """返回 (rddPostman 窗口句柄, Postman 窗口句柄)，找不到为 None。"""
    rows = []

    def cb(hwnd, lparam):
        try:
            if not u.IsWindowVisible(hwnd):
                return True
            n = u.GetWindowTextLengthW(hwnd)
            if n <= 0:
                return True
            buf = ctypes.create_unicode_buffer(n + 1)
            u.GetWindowTextW(hwnd, buf, n + 1)
            pid = wintypes.DWORD()
            u.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
            rows.append((hwnd, pid.value, buf.value))
        except Exception:
            pass  # 枚举绝不因单个窗口异常而中断
        return True

    CMPFUNC = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
    u.EnumWindows(CMPFUNC(cb), 0)

    app = pm = None
    for hwnd, pid, _title in rows:
        exe = process_image(pid)
        if exe.endswith('rddpostman.exe') and app is None:
            app = hwnd
        elif exe.endswith('postman.exe') and pm is None:
            pm = hwnd
    return app, pm


def overlap(a, b):
    return not (a.Right <= b.Left or b.Right <= a.Left or
                a.Bottom <= b.Top or b.Bottom <= a.Top)


def main():
    # 等待 app 窗口出现（手动启动后自动对齐），最多 90 秒
    hApp = hPm = None
    for _ in range(45):
        hApp, hPm = find_windows()
        if hApp and hPm:
            break
        time.sleep(2)
    if not hApp:
        print('90 秒内未出现 rddPostman 窗口；启动 app 后再运行本脚本即可')
        return
    if not hPm:
        print('未找到 Postman 窗口，请先启动 Postman 再运行本脚本')
        return

    rA, rP = rect(hApp), rect(hPm)
    top = rP.Top
    wA = rA.Right - rA.Left
    # app 紧贴 Postman 左侧（空间不足则贴右侧），顶边对齐，只移动不缩放
    xA = rP.Left - wA if rP.Left - wA >= 0 else rP.Right
    u.SetWindowPos(hApp, None, xA, top, 0, 0, 0x0001 | 0x0004)

    rA, rP = rect(hApp), rect(hPm)
    print(f'rddPostman: L={rA.Left} T={rA.Top} W={rA.Right - rA.Left} H={rA.Bottom - rA.Top}')
    print(f'Postman:    L={rP.Left} T={rP.Top} W={rP.Right - rP.Left} H={rP.Bottom - rP.Top}')
    print('顶边相同:', rA.Top == rP.Top, '| 左右紧挨:', rA.Right == rP.Left)


if __name__ == '__main__':
    main()
