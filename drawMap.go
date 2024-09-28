package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"io"
	"strings"
)

func getMapPreviewWithColors(hash string, slotColors [10]int) (image.Image, error) {
	buildingSizes := map[string][2]int{
		"A0ADemolishStructure":     {1, 1},
		"A0BaBaBunker":             {1, 1},
		"A0BaBaFactory":            {2, 1},
		"A0BaBaFlameTower":         {1, 1},
		"A0BaBaGunTower":           {1, 1},
		"A0BaBaGunTowerEND":        {1, 1},
		"A0BaBaHorizontalWall":     {1, 1},
		"A0BaBaMortarPit":          {1, 1},
		"A0BaBaPowerGenerator":     {1, 1},
		"A0BaBaRocketPit":          {1, 1},
		"A0BaBaRocketPitAT":        {1, 1},
		"A0BabaCornerWall":         {1, 1},
		"A0CannonTower":            {1, 1},
		"A0ComDroidControl":        {2, 2},
		"A0CommandCentre":          {2, 2},
		"A0CommandCentreCO":        {2, 2},
		"A0CommandCentreNE":        {2, 2},
		"A0CommandCentreNP":        {2, 2},
		"A0CyborgFactory":          {1, 2},
		"A0FacMod1":                {3, 3},
		"A0HardcreteMk1CWall":      {1, 1},
		"A0HardcreteMk1Wall":       {1, 1},
		"A0LightFactory":           {3, 3},
		"A0PowMod1":                {2, 2},
		"A0PowerGenerator":         {2, 2},
		"A0RepairCentre3":          {1, 1},
		"A0ResearchFacility":       {2, 2},
		"A0ResearchModule1":        {2, 2},
		"A0ResourceExtractor":      {1, 1},
		"A0TankTrap":               {1, 1},
		"A0VTolFactory1":           {3, 3},
		"A0VtolPad":                {1, 1},
		"AASite-QuadBof":           {1, 1},
		"AASite-QuadMg1":           {1, 1},
		"AASite-QuadRotMg":         {1, 1},
		"CO-Tower-HVCan":           {1, 1},
		"CO-Tower-HvATRkt":         {1, 1},
		"CO-Tower-HvFlame":         {1, 1},
		"CO-Tower-LtATRkt":         {1, 1},
		"CO-Tower-MG3":             {1, 1},
		"CO-Tower-MdCan":           {1, 1},
		"CO-Tower-RotMG":           {1, 1},
		"CO-WallTower-HvCan":       {1, 1},
		"CO-WallTower-RotCan":      {1, 1},
		"CollectiveCWall":          {1, 1},
		"CollectiveWall":           {1, 1},
		"CoolingTower":             {1, 1},
		"Emplacement-HPVcannon":    {1, 1},
		"Emplacement-Howitzer105":  {1, 1},
		"Emplacement-Howitzer150":  {1, 1},
		"Emplacement-HvART-pit":    {1, 1},
		"Emplacement-HvyATrocket":  {1, 1},
		"Emplacement-MRL-pit":      {1, 1},
		"Emplacement-MdART-pit":    {1, 1},
		"Emplacement-MortarPit01":  {1, 1},
		"Emplacement-MortarPit02":  {1, 1},
		"Emplacement-PrisLas":      {1, 1},
		"Emplacement-PulseLaser":   {1, 1},
		"Emplacement-Rail2":        {1, 1},
		"Emplacement-Rail3":        {1, 1},
		"Emplacement-Rocket06-IDF": {1, 1},
		"Emplacement-RotHow":       {1, 1},
		"Emplacement-RotMor":       {1, 1},
		"GuardTower-ATMiss":        {1, 1},
		"GuardTower-BeamLas":       {1, 1},
		"GuardTower-Rail1":         {1, 1},
		"GuardTower-RotMg":         {1, 1},
		"GuardTower1":              {1, 1},
		"GuardTower1MG":            {1, 1},
		"GuardTower2":              {1, 1},
		"GuardTower3":              {1, 1},
		"GuardTower3H":             {1, 1},
		"GuardTower4":              {1, 1},
		"GuardTower4H":             {1, 1},
		"GuardTower5":              {1, 1},
		"GuardTower6":              {1, 1},
		"LookOutTower":             {1, 1},
		"NEXUSCWall":               {1, 1},
		"NEXUSWall":                {1, 1},
		"NX-ANTI-SATSite":          {1, 1},
		"NX-CruiseSite":            {1, 1},
		"NX-Emp-MedArtMiss-Pit":    {1, 1},
		"NX-Emp-MultiArtMiss-Pit":  {1, 1},
		"NX-Emp-Plasma-Pit":        {1, 1},
		"NX-Tower-ATMiss":          {1, 1},
		"NX-Tower-PulseLas":        {1, 1},
		"NX-Tower-Rail1":           {1, 1},
		"NX-WallTower-BeamLas":     {1, 1},
		"NX-WallTower-Rail2":       {1, 1},
		"NX-WallTower-Rail3":       {1, 1},
		"NuclearReactor":           {2, 2},
		"P0-AASite-SAM1":           {1, 1},
		"P0-AASite-SAM2":           {1, 1},
		"PillBox1":                 {1, 1},
		"PillBox2":                 {1, 1},
		"PillBox3":                 {1, 1},
		"PillBox4":                 {1, 1},
		"PillBox5":                 {1, 1},
		"PillBox6":                 {1, 1},
		"Pillbox-RotMG":            {1, 1},
		"Sys-CB-Tower01":           {1, 1},
		"Sys-NEXUSLinkTOW":         {1, 1},
		"Sys-NX-CBTower":           {1, 1},
		"Sys-NX-SensorTower":       {1, 1},
		"Sys-NX-VTOL-CB-Tow":       {1, 1},
		"Sys-NX-VTOL-RadTow":       {1, 1},
		"Sys-SensoTower01":         {1, 1},
		"Sys-SensoTower02":         {1, 1},
		"Sys-VTOL-CB-Tower01":      {1, 1},
		"Sys-VTOL-RadarTower01":    {1, 1},
		"TankTrapC":                {1, 1},
		"Tower-Projector":          {1, 1},
		"Tower-RotMg":              {1, 1},
		"Tower-VulcanCan":          {1, 1},
		"UplinkCentre":             {2, 2},
		"Wall-RotMg":               {1, 1},
		"Wall-VulcanCan":           {1, 1},
		"WallTower-Atmiss":         {1, 1},
		"WallTower-HPVcannon":      {1, 1},
		"WallTower-HvATrocket":     {1, 1},
		"WallTower-Projector":      {1, 1},
		"WallTower-PulseLas":       {1, 1},
		"WallTower-Rail2":          {1, 1},
		"WallTower-Rail3":          {1, 1},
		"WallTower01":              {1, 1},
		"WallTower02":              {1, 1},
		"WallTower03":              {1, 1},
		"WallTower04":              {1, 1},
		"WallTower05":              {1, 1},
		"WallTower06":              {1, 1},
		"WreckedTransporter":       {3, 3},
	}
	buildingSizesMp := map[string][2]int{
		"A0ADemolishStructure":              {1, 1},
		"A0BaBaVtolPad":                     {1, 1},
		"bbaatow":                           {1, 1},
		"A0BaBaVtolFactory":                 {2, 2},
		"ScavRepairCentre":                  {1, 1},
		"A0BaBaBunker":                      {1, 1},
		"A0BaBaFactory":                     {2, 1},
		"A0BaBaFlameTower":                  {1, 1},
		"A0BaBaGunTower":                    {1, 1},
		"A0BaBaGunTowerEND":                 {1, 1},
		"A0BaBaHorizontalWall":              {1, 1},
		"A0BaBaMortarPit":                   {1, 1},
		"A0BaBaPowerGenerator":              {1, 1},
		"A0BaBaRocketPit":                   {1, 1},
		"A0BaBaRocketPitAT":                 {1, 1},
		"A0BabaCornerWall":                  {1, 1},
		"A0CannonTower":                     {1, 1},
		"A0ComDroidControl":                 {2, 2},
		"A0CommandCentre":                   {2, 2},
		"A0CyborgFactory":                   {1, 2},
		"A0FacMod1":                         {3, 3},
		"A0HardcreteMk1CWall":               {1, 1},
		"A0HardcreteMk1Gate":                {1, 1},
		"A0HardcreteMk1Wall":                {1, 1},
		"A0LasSatCommand":                   {2, 2},
		"A0LightFactory":                    {3, 3},
		"A0PowMod1":                         {2, 2},
		"A0PowerGenerator":                  {2, 2},
		"A0RepairCentre3":                   {1, 1},
		"A0ResearchFacility":                {2, 2},
		"A0ResearchModule1":                 {2, 2},
		"A0ResourceExtractor":               {1, 1},
		"A0Sat-linkCentre":                  {2, 2},
		"A0TankTrap":                        {1, 1},
		"A0VTolFactory1":                    {3, 3},
		"A0VtolPad":                         {1, 1},
		"AASite-QuadBof":                    {1, 1},
		"AASite-QuadBof02":                  {1, 1},
		"AASite-QuadMg1":                    {1, 1},
		"AASite-QuadRotMg":                  {1, 1},
		"CO-Tower-HVCan":                    {1, 1},
		"CO-Tower-HvATRkt":                  {1, 1},
		"CO-Tower-HvFlame":                  {1, 1},
		"CO-Tower-LtATRkt":                  {1, 1},
		"CO-Tower-MG3":                      {1, 1},
		"CO-Tower-MdCan":                    {1, 1},
		"CO-Tower-RotMG":                    {1, 1},
		"CO-WallTower-HvCan":                {1, 1},
		"CO-WallTower-RotCan":               {1, 1},
		"CollectiveCWall":                   {1, 1},
		"CollectiveWall":                    {1, 1},
		"CoolingTower":                      {1, 1},
		"ECM1PylonMk1":                      {1, 1},
		"Emplacement-HPVcannon":             {1, 1},
		"Emplacement-HeavyLaser":            {1, 1},
		"Emplacement-Howitzer-Incendiary":   {1, 1},
		"Emplacement-Howitzer-Incenediary":  {1, 1},
		"Emplacement-Howitzer105":           {1, 1},
		"Emplacement-Howitzer150":           {1, 1},
		"Emplacement-HvART-pit":             {1, 1},
		"Emplacement-HvyATrocket":           {1, 1},
		"Emplacement-MRL-pit":               {1, 1},
		"Emplacement-MRLHvy-pit":            {1, 1},
		"Emplacement-MdART-pit":             {1, 1},
		"Emplacement-MortarEMP":             {1, 1},
		"Emplacement-MortarPit-Incendiary":  {1, 1},
		"Emplacement-MortarPit-Incenediary": {1, 1},
		"Emplacement-MortarPit01":           {1, 1},
		"Emplacement-MortarPit02":           {1, 1},
		"Emplacement-PlasmaCannon":          {1, 1},
		"Emplacement-HeavyPlasmaLauncher":   {1, 1},
		"Emplacement-PrisLas":               {1, 1},
		"Emplacement-PulseLaser":            {1, 1},
		"Emplacement-Rail2":                 {1, 1},
		"Emplacement-Rail3":                 {1, 1},
		"Emplacement-Rocket06-IDF":          {1, 1},
		"Emplacement-RotHow":                {1, 1},
		"Emplacement-RotMor":                {1, 1},
		"GuardTower-ATMiss":                 {1, 1},
		"GuardTower-BeamLas":                {1, 1},
		"GuardTower-Rail1":                  {1, 1},
		"GuardTower-RotMg":                  {1, 1},
		"GuardTower1":                       {1, 1},
		"GuardTower2":                       {1, 1},
		"GuardTower3":                       {1, 1},
		"GuardTower4":                       {1, 1},
		"GuardTower5":                       {1, 1},
		"GuardTower6":                       {1, 1},
		"LookOutTower":                      {1, 1},
		"NEXUSCWall":                        {1, 1},
		"NEXUSWall":                         {1, 1},
		"NX-ANTI-SATSite":                   {1, 1},
		"NX-CruiseSite":                     {1, 1},
		"NX-Emp-MedArtMiss-Pit":             {1, 1},
		"NX-Emp-MultiArtMiss-Pit":           {1, 1},
		"NX-Emp-Plasma-Pit":                 {1, 1},
		"NX-Tower-ATMiss":                   {1, 1},
		"NX-Tower-PulseLas":                 {1, 1},
		"NX-Tower-Rail1":                    {1, 1},
		"NX-WallTower-BeamLas":              {1, 1},
		"NX-WallTower-Rail2":                {1, 1},
		"NX-WallTower-Rail3":                {1, 1},
		"NuclearReactor":                    {2, 2},
		"P0-AASite-Laser":                   {1, 1},
		"P0-AASite-SAM1":                    {1, 1},
		"P0-AASite-SAM2":                    {1, 1},
		"P0-AASite-Sunburst":                {1, 1},
		"PillBox-Cannon6":                   {1, 1},
		"PillBox1":                          {1, 1},
		"PillBox2":                          {1, 1},
		"PillBox3":                          {1, 1},
		"PillBox4":                          {1, 1},
		"PillBox5":                          {1, 1},
		"PillBox6":                          {1, 1},
		"Pillbox-RotMG":                     {1, 1},
		"Plasmite-flamer-bunker":            {1, 1},
		"Sys-CB-Tower01":                    {1, 1},
		"Sys-NEXUSLinkTOW":                  {1, 1},
		"Sys-NX-CBTower":                    {1, 1},
		"Sys-NX-SensorTower":                {1, 1},
		"Sys-NX-VTOL-CB-Tow":                {1, 1},
		"Sys-NX-VTOL-RadTow":                {1, 1},
		"Sys-RadarDetector01":               {1, 1},
		"Sys-SensoTower01":                  {1, 1},
		"Sys-SensoTower02":                  {1, 1},
		"Sys-SensoTowerWS":                  {1, 1},
		"Sys-SpyTower":                      {1, 1},
		"Sys-VTOL-CB-Tower01":               {1, 1},
		"Sys-VTOL-RadarTower01":             {1, 1},
		"TankTrapC":                         {1, 1},
		"Tower-Projector":                   {1, 1},
		"Tower-RotMg":                       {1, 1},
		"Tower-VulcanCan":                   {1, 1},
		"UplinkCentre":                      {2, 2},
		"Wall-RotMg":                        {1, 1},
		"Wall-VulcanCan":                    {1, 1},
		"WallTower-Atmiss":                  {1, 1},
		"WallTower-DoubleAAGun":             {1, 1},
		"WallTower-DoubleAAGun02":           {1, 1},
		"WallTower-EMP":                     {1, 1},
		"WallTower-HPVcannon":               {1, 1},
		"WallTower-HvATrocket":              {1, 1},
		"WallTower-Projector":               {1, 1},
		"WallTower-PulseLas":                {1, 1},
		"WallTower-QuadRotAAGun":            {1, 1},
		"WallTower-Rail2":                   {1, 1},
		"WallTower-Rail3":                   {1, 1},
		"WallTower-SamHvy":                  {1, 1},
		"WallTower-SamSite":                 {1, 1},
		"WallTower-TwinAssaultGun":          {1, 1},
		"WallTower01":                       {1, 1},
		"WallTower02":                       {1, 1},
		"WallTower03":                       {1, 1},
		"WallTower04":                       {1, 1},
		"WallTower05":                       {1, 1},
		"WallTower06":                       {1, 1},
		"WreckedTransporter":                {3, 3},
		"X-Super-Cannon":                    {2, 2},
		"X-Super-MassDriver":                {2, 2},
		"X-Super-Missile":                   {2, 2},
		"X-Super-Rocket":                    {2, 2},
	}
	for k, v := range buildingSizesMp {
		buildingSizes[k] = v
	}
	routimg, err := mapsdbGetTerrain(hash)
	if err != nil {
		return nil, err
	}
	outimg := imageToRGBA(routimg)

	mapblob, err := mapsdbGetBlob(hash)
	if err != nil {
		return nil, err
	}

	mr, err := zip.NewReader(bytes.NewReader(mapblob), int64(len(mapblob)))
	if err != nil {
		return nil, err
	}
	type object struct {
		Name     string `json:"name"`
		Position []int  `json:"position"`
		Rotation int    `json:"rotation"`
		Startpos int    `json:"startpos"`
		Player   string `json:"player"`
	}
	buildings := []object{}
	oils := []object{}
	droids := []object{}
	openread := func(v *zip.File, to any) error {
		fr, err := v.Open()
		if err != nil {
			return err
		}
		fb, err := io.ReadAll(fr)
		if err != nil {
			return err
		}
		return json.Unmarshal(fb, &to)
	}
	for _, v := range mr.File {
		if strings.HasSuffix(v.Name, "struct.json") {
			var fs struct {
				Structures []object `json:"structures"`
			}
			err := openread(v, &fs)
			if err != nil {
				return nil, err
			}
			buildings = fs.Structures
		}
		if strings.HasSuffix(v.Name, "feature.json") {
			var fs struct {
				Features []object `json:"features"`
			}
			err := openread(v, &fs)
			if err != nil {
				return nil, err
			}
			oils = fs.Features
		}
		if strings.HasSuffix(v.Name, "droid.json") {
			var fs struct {
				Droids []object `json:"droids"`
			}
			err := openread(v, &fs)
			if err != nil {
				return nil, err
			}
			droids = fs.Droids
		}
	}
	for _, b := range buildings {
		bs, ok := buildingSizes[b.Name]
		if !ok {
			continue
		}
		cl := playerColors[slotColors[b.Startpos]]
		if b.Player == "scavenger" {
			cl = color.RGBA{R: 128, A: 255}
		}

		outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128, cl)
		if bs[0] == 2 && bs[1] == 2 {
			outimg.SetRGBA(b.Position[0]/128-1, b.Position[1]/128-1, cl)
			outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128-1, cl)
			outimg.SetRGBA(b.Position[0]/128-1, b.Position[1]/128, cl)
			continue
		} else if bs[0] == 3 && bs[1] == 3 {
			outimg.SetRGBA(b.Position[0]/128-1, b.Position[1]/128-1, cl)
			outimg.SetRGBA(b.Position[0]/128-1, b.Position[1]/128, cl)
			outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128-1, cl)
			outimg.SetRGBA(b.Position[0]/128+1, b.Position[1]/128+1, cl)
			outimg.SetRGBA(b.Position[0]/128+1, b.Position[1]/128-1, cl)
			outimg.SetRGBA(b.Position[0]/128-1, b.Position[1]/128+1, cl)
			outimg.SetRGBA(b.Position[0]/128+1, b.Position[1]/128, cl)
			outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128+1, cl)
			continue
		} else if bs[0] == 1 && bs[1] == 2 {
			outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128-1, cl)
			continue
		}
	}
	for _, b := range oils {
		if b.Name != "OilResource" {
			continue
		}
		outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128, color.RGBA{R: 200, G: 200, B: 0, A: 255})
	}
	for _, b := range droids {
		cl := playerColors[slotColors[b.Startpos]]
		if b.Player == "scavenger" {
			cl = color.RGBA{R: 128, A: 255}
		}
		outimg.SetRGBA(b.Position[0]/128, b.Position[1]/128, cl)
	}
	return outimg, nil
}

func imageToRGBA(from image.Image) *image.RGBA {
	if ret, ok := from.(*image.RGBA); ok {
		return ret
	}
	b := from.Bounds()
	m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(m, m.Bounds(), from, b.Min, draw.Src)
	return m
}
