// Built by the Windows .NET Framework compiler; no external packages required.
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

class WindowsProcess
{
    [StructLayout(LayoutKind.Sequential)]
    struct BasicLimits
    {
        public long ProcessTime, JobTime;
        public uint Flags;
        public UIntPtr MinimumWorkingSet, MaximumWorkingSet;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint Priority, Scheduling;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct ExtendedLimits
    {
        public BasicLimits Basic;
        public ulong ReadOperations, WriteOperations, OtherOperations;
        public ulong ReadBytes, WriteBytes, OtherBytes;
        public UIntPtr ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct Accounting
    {
        public long UserTime, KernelTime, PeriodUserTime, PeriodKernelTime;
        public uint PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses;
    }
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern IntPtr CreateJobObject(IntPtr attributes, string name);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr job, int kind, ref ExtendedLimits limits, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool QueryInformationJobObject(IntPtr job, int kind, out Accounting info, uint size, IntPtr returned);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll")]
    static extern IntPtr GetCurrentProcess();
    [DllImport("kernel32.dll")]
    static extern bool CloseHandle(IntPtr handle);

    // The Windows CRT quoting convention, including empty arguments and slashes
    // immediately before quotes or the terminating quote.
    static string Quote(string value)
    {
        StringBuilder result = new StringBuilder("\"");
        int slashes = 0;
        foreach (char character in value)
        {
            if (character == '\\') { slashes++; continue; }
            if (character == '"') result.Append('\\', slashes * 2 + 1);
            else result.Append('\\', slashes);
            result.Append(character);
            slashes = 0;
        }
        result.Append('\\', slashes * 2);
        return result.Append('"').ToString();
    }

    static int Main(string[] args)
    {
        IntPtr job = IntPtr.Zero;
        try
        {
            if (String.Equals(Path.GetFileNameWithoutExtension(Environment.GetCommandLineArgs()[0]), "codex", StringComparison.OrdinalIgnoreCase))
            {
                string python = Environment.GetEnvironmentVariable("TOFA_LIVE_PYTHON");
                string harness = Environment.GetEnvironmentVariable("TOFA_LIVE_HARNESS");
                if (String.IsNullOrEmpty(python) || String.IsNullOrEmpty(harness))
                    throw new InvalidOperationException();
                List<string> observed = new List<string>();
                observed.Add(python);
                observed.Add(harness);
                observed.Add("--observe");
                observed.AddRange(args);
                args = observed.ToArray();
            }
            if (args.Length == 0) throw new InvalidOperationException();
            // No SECURITY_ATTRIBUTES means this handle is never inherited.
            job = CreateJobObject(IntPtr.Zero, null);
            if (job == IntPtr.Zero) throw new InvalidOperationException();
            ExtendedLimits limits = new ExtendedLimits();
            limits.Basic.Flags = 0x2000; // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
            if (!SetInformationJobObject(job, 9, ref limits, (uint)Marshal.SizeOf(typeof(ExtendedLimits)))
                || !AssignProcessToJobObject(job, GetCurrentProcess()))
                throw new InvalidOperationException();

            // Joining the job before spawning removes the child-assignment race.
            ProcessStartInfo start = new ProcessStartInfo();
            start.FileName = args[0];
            List<string> quoted = new List<string>();
            for (int index = 1; index < args.Length; index++) quoted.Add(Quote(args[index]));
            start.Arguments = String.Join(" ", quoted.ToArray());
            start.UseShellExecute = false;
            int status;
            using (Process child = Process.Start(start))
            {
                child.WaitForExit();
                status = child.ExitCode;
            }
            // Uninstallers may leave a helper running after their entrypoint exits.
            for (;;)
            {
                Accounting info;
                if (!QueryInformationJobObject(job, 1, out info, (uint)Marshal.SizeOf(typeof(Accounting)), IntPtr.Zero))
                    throw new InvalidOperationException();
                if (info.ActiveProcesses <= 1) break;
                Thread.Sleep(25);
            }
            limits.Basic.Flags = 0;
            if (!SetInformationJobObject(job, 9, ref limits, (uint)Marshal.SizeOf(typeof(ExtendedLimits))))
                throw new InvalidOperationException();
            CloseHandle(job);
            job = IntPtr.Zero;
            return status;
        }
        catch
        {
            // Never print exception messages: command arguments may contain secrets.
            Console.Error.WriteLine("Windows process supervisor failed (125).");
            // Returning closes the job handle and terminates any remaining children.
            return 125;
        }
    }
}
