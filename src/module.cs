using System.IO;
using System.Reflection;
using SDG.Framework.Modules;
using SDG.Provider;
using SDG.Unturned;

using static SDG.Unturned.Logs;

namespace BypassCountryRestrictions
{
    public class Main : IModuleNexus
    {
        public static string FlagPath = Path.Combine(
            Directory.GetCurrentDirectory(), "Modules", "BypassCountryRestrictions", "Flag");

        public void initialize()
        {
            File.WriteAllText(FlagPath, "false");
            printLine("Overriding Country Restrictions...");

            typeof(TempSteamworksEconomy).GetProperty("hasCountryDetails",
                BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.Public
            ).SetValue(Provider.provider.economyService, true);

            typeof(TempSteamworksEconomy).GetProperty("doesCountryAllowRandomItems",
                BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.Public
            ).SetValue(Provider.provider.economyService, true);

            printLine("Override Complete.");
        }

        public void shutdown()
        {
            printLine("Writing flag...");
            File.WriteAllText(FlagPath, "true");
            printLine("Unloaded Country Restrictions Bypass");
        }
    }
}
