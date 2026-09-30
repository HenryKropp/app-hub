# Shows the modern Windows folder picker (the Explorer-style dialog), sharp on
# high-DPI screens and on top of other windows, and prints the chosen folder
# as UTF-8. Input: $env:APPHUB_INITIAL (start folder). With $env:APPHUB_CHECK
# set it only compiles the helper and prints "ok" (used to test the script).
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -ReferencedAssemblies System.Windows.Forms -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Windows.Forms;

public static class AppHubFolderPicker {
    [DllImport("user32.dll")] static extern bool SetProcessDPIAware();

    [ComImport, Guid("DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7")]
    class FileOpenDialog {}

    [ComImport, Guid("42f85136-db7e-439c-85f1-e4075d135fc8"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IFileDialog {
        [PreserveSig] int Show(IntPtr parent);
        void SetFileTypes(uint count, IntPtr specs);
        void SetFileTypeIndex(uint index);
        void GetFileTypeIndex(out uint index);
        void Advise(IntPtr events, out uint cookie);
        void Unadvise(uint cookie);
        void SetOptions(uint options);
        void GetOptions(out uint options);
        void SetDefaultFolder(IShellItem item);
        void SetFolder(IShellItem item);
        void GetFolder(out IShellItem item);
        void GetCurrentSelection(out IShellItem item);
        void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string name);
        void GetFileName(out IntPtr name);
        void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string title);
        void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string label);
        void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string label);
        void GetResult(out IShellItem item);
    }

    [ComImport, Guid("43826D1E-E718-42EE-BC55-A1E261C37BFE"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IShellItem {
        void BindToHandler(IntPtr bc, ref Guid bhid, ref Guid riid, out IntPtr ppv);
        void GetParent(out IShellItem parent);
        void GetDisplayName(uint sigdn, [MarshalAs(UnmanagedType.LPWStr)] out string name);
        void GetAttributes(uint mask, out uint attributes);
        void Compare(IShellItem other, uint hint, out int order);
    }

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
    static extern void SHCreateItemFromParsingName(string path, IntPtr bc, [In] ref Guid riid, out IShellItem item);

    const uint FOS_PICKFOLDERS = 0x20, FOS_FORCEFILESYSTEM = 0x40;
    const uint SIGDN_FILESYSPATH = 0x80058000;

    public static void Init() { SetProcessDPIAware(); Application.EnableVisualStyles(); }

    // Returns the chosen folder, or "" when canceled.
    public static string Pick(string initial) {
        using (var owner = new Form { TopMost = true, ShowInTaskbar = false, Opacity = 0, StartPosition = FormStartPosition.CenterScreen }) {
            owner.Show();
            owner.Activate();
            var dialog = (IFileDialog)new FileOpenDialog();
            uint options;
            dialog.GetOptions(out options);
            dialog.SetOptions(options | FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM);
            dialog.SetTitle("Onde salvar os resultados");
            dialog.SetOkButtonLabel("Usar esta pasta");
            if (!string.IsNullOrEmpty(initial)) {
                try {
                    Guid iid = typeof(IShellItem).GUID;
                    IShellItem start;
                    SHCreateItemFromParsingName(initial, IntPtr.Zero, ref iid, out start);
                    dialog.SetFolder(start);
                } catch { }
            }
            if (dialog.Show(owner.Handle) != 0) return "";
            IShellItem result;
            dialog.GetResult(out result);
            string path;
            result.GetDisplayName(SIGDN_FILESYSPATH, out path);
            return path;
        }
    }
}
'@
if ($env:APPHUB_CHECK) { [Console]::Out.Write('ok'); exit }
[AppHubFolderPicker]::Init()
[Console]::Out.Write([AppHubFolderPicker]::Pick($env:APPHUB_INITIAL))
