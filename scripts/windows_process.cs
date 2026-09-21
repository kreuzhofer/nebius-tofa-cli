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
    [StructLayout(LayoutKind.Sequential)]
    struct CompletionPort
    {
        public IntPtr Key, Port;
    }
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern IntPtr CreateJobObject(IntPtr attributes, string name);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr job, int kind, ref ExtendedLimits limits, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr job, int kind, ref CompletionPort port, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool QueryInformationJobObject(IntPtr job, int kind, out Accounting info, uint size, IntPtr returned);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll")]
    static extern IntPtr GetCurrentProcess();
    [DllImport("kernel32.dll")]
    static extern uint GetCurrentProcessId();
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr CreateIoCompletionPort(IntPtr file, IntPtr port, UIntPtr key, uint threads);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetQueuedCompletionStatus(IntPtr port, out uint message, out UIntPtr key, out IntPtr value, uint timeout);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr OpenProcess(uint access, bool inherit, uint processId);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool IsProcessInJob(IntPtr process, IntPtr job, out bool result);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetExitCodeProcess(IntPtr process, out uint status);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint WaitForSingleObject(IntPtr handle, uint timeout);
    [DllImport("kernel32.dll")]
    static extern bool CloseHandle(IntPtr handle);

    static bool ObserveProcess(IntPtr port, IntPtr job, int entrypoint, Dictionary<uint, IntPtr> descendants, uint timeout)
    {
        uint message;
        UIntPtr key;
        IntPtr value;
        if (!GetQueuedCompletionStatus(port, out message, out key, out value, timeout))
        {
            if (Marshal.GetLastWin32Error() == 258) return false; // WAIT_TIMEOUT
            throw new InvalidOperationException();
        }
        if (message != 6) return true; // JOB_OBJECT_MSG_NEW_PROCESS
        uint pid = unchecked((uint)value.ToInt64());
        if (pid == GetCurrentProcessId() || pid == unchecked((uint)entrypoint)) return true;
        // Retain each handle until verification: a notification's PID can
        // otherwise be recycled before its exit status is read.
        if (descendants.ContainsKey(pid)) throw new InvalidOperationException();
        IntPtr process = OpenProcess(0x101000, false, pid); // SYNCHRONIZE | PROCESS_QUERY_LIMITED_INFORMATION
        if (process == IntPtr.Zero) throw new InvalidOperationException();
        bool member;
        if (!IsProcessInJob(process, job, out member) || !member)
        {
            CloseHandle(process);
            throw new InvalidOperationException();
        }
        descendants.Add(pid, process);
        return true;
    }

    static void InspectExitedProcesses(Dictionary<uint, IntPtr> descendants)
    {
        foreach (uint pid in new List<uint>(descendants.Keys))
        {
            IntPtr process = descendants[pid];
            if (process == IntPtr.Zero) continue;
            uint wait = WaitForSingleObject(process, 0);
            if (wait == 258) continue; // WAIT_TIMEOUT: still running
            uint status;
            if (wait != 0 || !GetExitCodeProcess(process, out status) || status != 0)
                throw new InvalidOperationException();
            // Release terminated processes before waiting for job accounting
            // to drain. Keep their PID as evidence; any reuse fails closed.
            CloseHandle(process);
            descendants[pid] = IntPtr.Zero;
        }
    }

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
        IntPtr port = IntPtr.Zero;
        Dictionary<uint, IntPtr> descendants = new Dictionary<uint, IntPtr>();
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
            bool requireSuccess = args[0] == "--require-descendant-success";
            if (requireSuccess)
            {
                string[] command = new string[args.Length - 1];
                Array.Copy(args, 1, command, 0, command.Length);
                args = command;
                if (args.Length == 0) throw new InvalidOperationException();
            }
            // No SECURITY_ATTRIBUTES means this handle is never inherited.
            job = CreateJobObject(IntPtr.Zero, null);
            if (job == IntPtr.Zero) throw new InvalidOperationException();
            if (requireSuccess)
            {
                port = CreateIoCompletionPort(new IntPtr(-1), IntPtr.Zero, UIntPtr.Zero, 1);
                if (port == IntPtr.Zero) throw new InvalidOperationException();
                CompletionPort association = new CompletionPort();
                association.Port = port;
                // Associate while empty, before any process can enter the job.
                if (!SetInformationJobObject(job, 7, ref association, (uint)Marshal.SizeOf(typeof(CompletionPort))))
                    throw new InvalidOperationException();
            }
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
            int status = 0;
            using (Process child = Process.Start(start))
            {
                int childId = child.Id;
                bool childExited = false;
                // Observe while the entrypoint is still running, so short-lived
                // helpers are not first inspected only after its exit.
                for (;;)
                {
                    if (requireSuccess)
                    {
                        ObserveProcess(port, job, childId, descendants, 25);
                        InspectExitedProcesses(descendants);
                    }
                    else Thread.Sleep(25);
                    if (!childExited && child.HasExited)
                    {
                        status = child.ExitCode;
                        child.Close();
                        childExited = true;
                    }
                    Accounting info;
                    if (!QueryInformationJobObject(job, 1, out info, (uint)Marshal.SizeOf(typeof(Accounting)), IntPtr.Zero))
                        throw new InvalidOperationException();
                    if (!childExited || info.ActiveProcesses > 1) continue;
                    if (requireSuccess)
                    {
                        while (ObserveProcess(port, job, childId, descendants, 0)) {}
                        InspectExitedProcesses(descendants);
                        // Job notifications are not guaranteed. A missing event,
                        // exited-before-open process, or reused PID must fail
                        // closed instead of treating incomplete evidence as zero.
                        if (info.TotalProcesses != descendants.Count + 2)
                            throw new InvalidOperationException();
                        foreach (IntPtr process in descendants.Values)
                        {
                            if (process != IntPtr.Zero) throw new InvalidOperationException();
                        }
                    }
                    break;
                }
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
        finally
        {
            foreach (IntPtr process in descendants.Values)
                if (process != IntPtr.Zero) CloseHandle(process);
            if (port != IntPtr.Zero) CloseHandle(port);
        }
    }
}
